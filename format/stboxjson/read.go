package stboxjson

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/phpkey"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

type reader struct {
	st *parsing.State
	// version is the version field of the document, 0 when it is missing or not an integer.
	version int64
}

type number struct {
	text json.Number
	f    float64
}

func (r *reader) fail(msg string, block int, entry any) error {
	var lines []string
	if r.st.Lenient() {
		lines = []string{encodeEntry(entry)}
	}
	return r.st.Fail(parseError(msg), model.NoLine, block, lines)
}

// sortWarnings sorts warnings by block with a stable sort, where NoBlock counts as block 0, as PHP sorts them.
// The warnings of the text pre-pass have no block and come first, so they stay first.
func sortWarnings(warnings []model.ParseWarning) []model.ParseWarning {
	slices.SortStableFunc(warnings, func(a, b model.ParseWarning) int { return cmp.Compare(max(a.Block, 0), max(b.Block, 0)) })
	return warnings
}

func (r *reader) read(content []byte) (*model.Subtitle, error) {
	root, err := decodeRoot(content)
	if err != nil {
		return nil, err
	}
	normalize(root)
	v, _ := root.Get("version")
	r.version, _ = integer(v)
	if err := r.decodeFileBinary(root); err != nil {
		return nil, err
	}
	cues, _ := root.List("cues")
	skipped := map[int]bool{}
	if err := r.decodeCueBinary(cues, skipped); err != nil {
		return nil, err
	}
	if err := r.checkCueTimes(cues, skipped); err != nil {
		return nil, err
	}
	return r.subtitle(root, skipped)
}

// normalize turns each list that PHP reads as an object into an object with the keys "0", "1" and so on:
// the metadata, the format data and the data of one format. A cue or comment that is a list stays one until its
// check, so a warning shows it as a list, as PHP does.
func normalize(root *model.Data) {
	listToObject(root, "metadata")
	normalizeFormatData(root)
	cues, _ := root.List("cues")
	for _, c := range cues {
		if cue, ok := c.(*model.Data); ok {
			normalizeFormatData(cue)
		}
	}
}

func normalizeFormatData(d *model.Data) {
	listToObject(d, "formatData")
	fd, _ := d.Object("formatData")
	for _, key := range fd.Keys() {
		listToObject(fd, key)
	}
}

func listToObject(d *model.Data, key string) {
	if v, ok := d.List(key); ok {
		d.Set(key, asObject(v))
	}
}

func asObject(v any) any {
	list, ok := v.([]any)
	if !ok {
		return v
	}
	obj := model.NewData()
	for i, item := range list {
		obj.Set(strconv.Itoa(i), item)
	}
	return obj
}

func isContainer(v any) bool {
	switch v.(type) {
	case []any, *model.Data:
		return true
	default:
		return false
	}
}

func (r *reader) decodeFileBinary(root *model.Data) error {
	fd, ok := root.Object("formatData")
	if !ok {
		return nil
	}
	kept := model.NewData()
	for _, key := range fd.Keys() {
		v, _ := fd.Get(key)
		decoded, msg := v, ""
		if isContainer(v) {
			decoded, msg = decodeBinary(v, "formatData."+key)
		}
		if msg == "" {
			kept.Set(key, decoded)
			continue
		}
		if err := r.fail(msg, model.NoBlock, model.NewData(model.Field{Key: key, Value: v})); err != nil {
			return err
		}
	}
	root.Set("formatData", kept)
	return nil
}

func (r *reader) decodeCueBinary(cues []any, skipped map[int]bool) error {
	for i, c := range cues {
		cue, ok := c.(*model.Data)
		if !ok {
			continue
		}
		fd, _ := cue.Get("formatData")
		if !isContainer(fd) {
			continue
		}
		decoded, msg := decodeBinary(fd, cuePath(i)+".formatData")
		if msg == "" {
			cue.Set("formatData", decoded)
			continue
		}
		if err := r.fail(msg, i, cue); err != nil {
			return err
		}
		skipped[i] = true
	}
	return nil
}

// decodeBinary returns a copy of v in which each object whose only key is base64 with a string value is the decoded
// string, or the message for the first value that is not valid base64.
func decodeBinary(v any, path string) (any, string) {
	switch v := v.(type) {
	case *model.Data:
		return decodeBinaryObject(v, path)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			decoded, msg := decodeBinary(item, path+"["+strconv.Itoa(i)+"]")
			if msg != "" {
				return nil, msg
			}
			out[i] = decoded
		}
		return out, ""
	default:
		return v, ""
	}
}

func decodeBinaryObject(d *model.Data, path string) (any, string) {
	if s, ok := d.String("base64"); ok && d.Len() == 1 {
		b, ok := decodeBase64(s)
		if !ok {
			return nil, "The field " + path + ".base64 must be valid base64."
		}
		return string(b), ""
	}
	out := model.NewData()
	for _, key := range d.Keys() {
		item, _ := d.Get(key)
		decoded, msg := decodeBinary(item, keyPath(path, key))
		if msg != "" {
			return nil, msg
		}
		out.Set(key, decoded)
	}
	return out, ""
}

// decodeBase64 decodes s as PHP base64_decode in strict mode does: it skips space, tab, CR and LF, and accepts
// the padding either complete or left out.
func decodeBase64(s string) ([]byte, bool) {
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, s)
	enc := base64.StdEncoding
	if !strings.Contains(s, "=") {
		enc = base64.RawStdEncoding
	}
	b, err := enc.DecodeString(s)
	return b, err == nil
}

// keyPath returns the path of key in the object at path. A key that PHP reads as an integer gets the list form.
func keyPath(path, key string) string {
	if phpkey.IsIntKey(key) {
		return path + "[" + key + "]"
	}
	return path + "." + key
}

func cuePath(i int) string {
	return "cues[" + strconv.Itoa(i) + "]"
}

func (r *reader) checkCueTimes(cues []any, skipped map[int]bool) error {
	for i, c := range cues {
		cue, ok := c.(*model.Data)
		if !ok || skipped[i] {
			continue
		}
		msg := r.checkTimes(cue, i)
		if msg == "" {
			continue
		}
		if err := r.fail(msg, i, cue); err != nil {
			return err
		}
		skipped[i] = true
	}
	return nil
}

func (r *reader) checkTimes(cue *model.Data, i int) string {
	start, startOK := finite(cue.Get("start"))
	end, endOK := finite(cue.Get("end"))
	if msg := checkBound(i, "start", start, startOK); msg != "" {
		return msg
	}
	if msg := checkBound(i, "end", end, endOK); msg != "" {
		return msg
	}
	if r.version == 1 {
		if msg := message(parsing.CheckWordTimestamps(stringLines(cue), model.NoLine)); msg != "" {
			return msg
		}
	}
	if !startOK || !endOK || end.f >= start.f {
		return ""
	}
	return r.orderTimes(cue, i, start, end)
}

// checkBound returns the message for a time that rounds to 100000 hours or more, or to -100000 hours or less.
func checkBound(i int, key string, n number, ok bool) string {
	if !ok {
		return ""
	}
	if _, ok := milliseconds(n); ok {
		return ""
	}
	if n.f < 0 {
		return "The field " + cuePath(i) + "." + key + " is not above -" + strconv.Itoa(timecode.MaxHours) + " hours."
	}
	_, err := parsing.BoundedField(timecode.MaxTime, cuePath(i)+"."+key)
	return message(err)
}

// orderTimes compares the float values, as PHP does, where parsing.State.OrderedTimes compares durations.
// Lenient mode swaps the times of a reversed cue that then lasts parsing.MaxSwappedDuration or less.
func (r *reader) orderTimes(cue *model.Data, i int, start, end number) string {
	msg := "The cue ends at " + timecode.Number(end.f) + " s, before it starts at " + timecode.Number(start.f) + " s."
	if !r.st.Lenient() || start.f-end.f > parsing.MaxSwappedDuration.Seconds() {
		return msg
	}
	r.st.Warn(msg+" The parser swapped the times.", model.NoLine, i, []string{encodeEntry(cue)}, model.Repaired)
	cue.Set("start", end.text)
	cue.Set("end", start.text)
	return ""
}

// stringLines returns the strings of the lines of cue. PHP checks their word timestamps before it checks the lines.
func stringLines(cue *model.Data) []string {
	lines, _ := cue.List("lines")
	var out []string
	for _, line := range lines {
		if s, ok := line.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func message(err error) string {
	var perr *model.ParseError
	if !errors.As(err, &perr) {
		return ""
	}
	return perr.Msg
}

// finite returns v as a number when it is a JSON number with a finite float64 value. PHP reads 1e400 as INF.
func finite(v any, _ bool) (number, bool) {
	text, ok := v.(json.Number)
	if !ok {
		return number{}, false
	}
	f, _ := strconv.ParseFloat(string(text), 64)
	return number{text, f}, !math.IsInf(f, 0)
}

// integer returns v when it is a number literal without fraction and exponent that fits an int64, as PHP reads an int.
func integer(v any) (int64, bool) {
	text, ok := v.(json.Number)
	if !ok || strings.ContainsAny(string(text), ".eE") {
		return 0, false
	}
	n, err := strconv.ParseInt(string(text), 10, 64)
	return n, err == nil
}

// milliseconds converts the decimal literal of n and rounds half away from zero, so 1.0005 gives 1001 ms.
// It fails for a time of 100000 hours or more, in either sign.
func milliseconds(n number) (time.Duration, bool) {
	v, err := orderedjson.NumberText(string(n.text))
	if err != nil {
		return 0, false
	}
	return v.Duration(time.Second)
}

func toModel(v any) any {
	switch v := v.(type) {
	case json.Number:
		if n, ok := integer(v); ok {
			return n
		}
		f, _ := strconv.ParseFloat(string(v), 64)
		return f
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = toModel(item)
		}
		return out
	case *model.Data:
		return toModelData(v)
	default:
		return v
	}
}

func toModelData(d *model.Data) *model.Data {
	out := model.NewData()
	for _, key := range d.Keys() {
		item, _ := d.Get(key)
		out.Set(key, toModel(item))
	}
	return out
}

// subtitle runs the document steps of PHP ArrayReader on the decoded tree.
func (r *reader) subtitle(root *model.Data, skipped map[int]bool) (*model.Subtitle, error) {
	if err := checkVersion(root); err != nil {
		return nil, err
	}
	s := &model.Subtitle{}
	if err := r.metadata(s, root); err != nil {
		return nil, err
	}
	if err := r.formatData(s, root); err != nil {
		return nil, err
	}
	list, ok := root.List("cues")
	if !ok {
		return nil, parseError("The field cues must be a list.")
	}
	cues, err := r.cues(list, skipped)
	if err != nil {
		return nil, err
	}

	// Cues in start order keep the stored comment indexes. Else each comment moves with the cue it stands before.
	inOrder := slices.IsSortedFunc(cues, func(a, b model.Cue) int { return cmp.Compare(a.Start, b.Start) })
	if inOrder {
		s.AddCues(cues...)
	}
	if err := r.comments(s, root, skipped); err != nil {
		return nil, err
	}
	if !inOrder {
		s.ReplaceCues(cues, nil)
	}
	return s, nil
}

func (r *reader) cues(list []any, skipped map[int]bool) ([]model.Cue, error) {
	var cues []model.Cue
	for i, entry := range list {
		if skipped[i] {
			continue
		}
		c, msg := r.cue(entry, i)
		if msg == "" {
			cues = append(cues, c)
			continue
		}
		if err := r.fail(msg, i, entry); err != nil {
			return nil, err
		}
		skipped[i] = true
	}
	return cues, nil
}

func checkVersion(root *model.Data) error {
	version, ok := root.Get("version")
	if !ok {
		return parseError("The field version is missing.")
	}
	if n, ok := integer(version); !ok || n != 1 && n != Version {
		return parseError("The field version must be 1 or 2.")
	}
	return nil
}

// object treats a missing key and null as an empty object, as the PHP ?? operator does.
func object(d *model.Data, key string) (*model.Data, bool) {
	v, _ := d.Get(key)
	switch v := v.(type) {
	case nil:
		return model.NewData(), true
	case *model.Data:
		return v, true
	default:
		return nil, false
	}
}

func (r *reader) metadata(s *model.Subtitle, root *model.Data) error {
	metadata, ok := object(root, "metadata")
	if !ok {
		return parseError("The field metadata must be an object.")
	}
	for _, key := range metadata.Keys() {
		v, _ := metadata.Get(key)
		if text, ok := v.(string); ok {
			s.SetMetadata(key, text)
			continue
		}
		if err := r.fail("The field metadata."+key+" must be a string.", model.NoBlock, model.NewData(model.Field{Key: key, Value: v})); err != nil {
			return err
		}
	}
	return nil
}

func (r *reader) formatData(s *model.Subtitle, root *model.Data) error {
	fd, ok := object(root, "formatData")
	if !ok {
		return parseError("The field formatData must be an object.")
	}
	for _, key := range fd.Keys() {
		v, _ := fd.Get(key)
		msg := "The field formatData." + key + " must be an object."
		if data, ok := v.(*model.Data); ok {
			msg = formatDataMessage(s.SetFormatData(key, toModelData(data)), "")
		}
		if msg == "" {
			continue
		}
		if err := r.fail(msg, model.NoBlock, model.NewData(model.Field{Key: key, Value: v})); err != nil {
			return err
		}
	}
	return nil
}

func formatDataMessage(err error, prefix string) string {
	var iae *model.InvalidArgumentError
	if !errors.As(err, &iae) {
		return ""
	}
	if rest, ok := strings.CutPrefix(iae.Msg, "The field "); ok && prefix != "" {
		return "The field " + prefix + "." + rest
	}
	return iae.Msg
}

func (r *reader) cue(entry any, i int) (model.Cue, string) {
	path := "The field " + cuePath(i)
	d, ok := asObject(entry).(*model.Data)
	if !ok {
		return model.Cue{}, path + " must be an object."
	}
	var c model.Cue
	checks := []func(*model.Cue, *model.Data, string) string{cueTimes, cueLines, cueSettings, cuePosition, cueForced}
	if r.version == Version {
		checks = append(checks, cueSpeaker, cueWords)
	}
	for _, check := range checks {
		if msg := check(&c, d, path); msg != "" {
			return model.Cue{}, msg
		}
	}
	if msg := cueFormatData(&c, d, i, r.version); msg != "" {
		return model.Cue{}, msg
	}
	if r.version == Version {
		if msg := validateWords(&c, path); msg != "" {
			return c, msg
		}
		return c, cueTagLayout(&c, d, i)
	}
	c.ExtractWordTags()
	return c, ""
}

func cueTimes(c *model.Cue, d *model.Data, path string) string {
	for _, t := range []struct {
		key string
		to  *time.Duration
	}{{"start", &c.Start}, {"end", &c.End}} {
		n, ok := finite(d.Get(t.key))
		if !ok {
			return path + "." + t.key + " must be a number."
		}
		*t.to, _ = milliseconds(n) // checkBound accepted the time
	}
	return ""
}

func cueLines(c *model.Cue, d *model.Data, path string) string {
	lines, ok := d.List("lines")
	if !ok {
		return path + ".lines must be a list."
	}
	c.Lines = make([]string, len(lines))
	for j, line := range lines {
		if c.Lines[j], ok = line.(string); !ok {
			return path + ".lines[" + strconv.Itoa(j) + "] must be a string."
		}
	}
	return ""
}

func cueSettings(c *model.Cue, d *model.Data, path string) string {
	switch id, _ := d.Get("identifier"); id := id.(type) {
	case nil:
	case string:
		c.ID = id
	default:
		return path + ".identifier must be a string or null."
	}
	if v, _ := d.Get("alignment"); v != nil {
		n, ok := integer(v)
		if !ok || n < 1 || n > 9 {
			return path + ".alignment must be an integer from 1 to 9 or null."
		}
		c.Position.Alignment, _ = model.AlignmentFromNumpad(int(n))
	}
	return ""
}

// validateKeys maps the Option of a Position.Validate error to the JSON key path below position.
var validateKeys = map[string]string{ //nolint:gochecknoglobals // a lookup table
	"Position.Row": ".row", "Position.Column": ".column", "Position.Grid": ".grid",
	"Position.Offset.X": ".offset.x", "Position.Offset.Y": ".offset.y",
}

func cuePosition(c *model.Cue, d *model.Data, path string) string {
	v, _ := d.Get("position")
	if v == nil {
		return ""
	}
	path += ".position"
	obj, ok := v.(*model.Data)
	if !ok {
		return path + " must be an object or null."
	}
	for _, f := range []struct {
		key string
		to  *int
	}{{"row", &c.Position.Row}, {"column", &c.Position.Column}} {
		if msg := optionalInt(obj, f.key, f.to, path); msg != "" {
			return msg
		}
	}
	if msg := positionGrid(&c.Position.Grid, obj, path); msg != "" {
		return msg
	}
	if msg := positionOffset(&c.Position, obj, path); msg != "" {
		return msg
	}
	var iae *model.InvalidArgumentError
	if errors.As(c.Position.Validate(), &iae) {
		return path + validateKeys[iae.Option] + " is out of range. " + iae.Msg
	}
	return ""
}

// optionalInt sets to from the integer d[key]. A missing key and null leave it at 0.
func optionalInt(d *model.Data, key string, to *int, path string) string {
	v, _ := d.Get(key)
	if v == nil {
		return ""
	}
	n, ok := integer(v)
	if !ok || n < math.MinInt32 || n > math.MaxInt32 {
		return path + "." + key + " must be an integer or null."
	}
	*to = int(n)
	return ""
}

func positionGrid(g *model.Grid, d *model.Data, path string) string {
	v, _ := d.Get("grid")
	if v == nil {
		return ""
	}
	obj, ok := v.(*model.Data)
	if !ok {
		return path + ".grid must be an object or null."
	}
	for _, f := range []struct {
		key string
		to  *int
	}{{"rows", &g.Rows}, {"columns", &g.Columns}} {
		if msg := optionalInt(obj, f.key, f.to, path+".grid"); msg != "" {
			return msg
		}
	}
	return ""
}

func positionOffset(p *model.Position, d *model.Data, path string) string {
	v, _ := d.Get("offset")
	if v == nil {
		return ""
	}
	obj, ok := v.(*model.Data)
	if !ok {
		return path + ".offset must be an object or null."
	}
	var o model.Offset
	for _, f := range []struct {
		key string
		to  *float64
	}{{"x", &o.X}, {"y", &o.Y}} {
		n, ok := finite(obj.Get(f.key))
		if !ok {
			return path + ".offset." + f.key + " must be a finite number."
		}
		*f.to = n.f
	}
	p.Offset = &o
	return ""
}

func cueForced(c *model.Cue, d *model.Data, path string) string {
	switch forced, _ := d.Get("forced"); forced := forced.(type) {
	case nil:
	case bool:
		c.Forced = forced
	default:
		return path + ".forced must be a boolean."
	}
	return ""
}

func cueFormatData(c *model.Cue, d *model.Data, i int, version int64) string {
	fd, ok := object(d, "formatData")
	if !ok {
		return "The field " + cuePath(i) + ".formatData must be an object."
	}
	for _, key := range fd.Keys() {
		v, _ := fd.Get(key)
		obj, ok := v.(*model.Data)
		if !ok {
			return "The field " + cuePath(i) + ".formatData." + key + " must be an object."
		}
		if key == taggedKey && version == Version {
			continue
		}
		if key == imageKey {
			if msg := cueImage(c, obj, cuePath(i)+".formatData.image"); msg != "" {
				return "The field " + msg
			}
			continue
		}
		data := toModelData(obj)
		addAlignment(data, key, c.Position.Alignment)
		if msg := formatDataMessage(c.SetFormatData(key, data), cuePath(i)); msg != "" {
			return msg
		}
	}
	return ""
}

// cueImage sets c.Image from the object d of the key image, and sets c.Forced when image.forced is true.
// An empty object is no image. The returned message starts with the path.
func cueImage(c *model.Cue, d *model.Data, path string) string {
	if d.Len() == 0 {
		return ""
	}
	var im model.Image
	if msg := imageFields(&im, d, path); msg != "" {
		return msg
	}
	switch forced, _ := d.Get("forced"); forced := forced.(type) {
	case nil:
	case bool:
		c.Forced = c.Forced || forced
	default:
		return path + ".forced must be a boolean."
	}
	if err := im.Validate(); err != nil {
		var iae *model.InvalidArgumentError
		_ = errors.As(err, &iae)
		return path + " is invalid. " + iae.Msg
	}
	c.Image = &im
	return ""
}

func imageFields(im *model.Image, d *model.Data, path string) string {
	for _, f := range []struct {
		key string
		to  *int
	}{
		{"x", &im.X},
		{"y", &im.Y},
		{"width", &im.Width},
		{"height", &im.Height},
		{"screenWidth", &im.ScreenWidth},
		{"screenHeight", &im.ScreenHeight},
	} {
		v, _ := d.Get(f.key)
		n, ok := integer(v)
		if !ok || n < math.MinInt32 || n > math.MaxInt32 {
			return path + "." + f.key + " must be an integer."
		}
		*f.to = int(n)
	}
	png, ok := d.String("png")
	if !ok {
		return path + ".png must be a string."
	}
	im.PNG = []byte(png)
	return ""
}

// addAlignment adds the numpad alignment to vtt and ttml cue data that has none. PHP 3.x stored no alignment there,
// and the writers drop a stored position without it.
func addAlignment(data *model.Data, key string, a model.Alignment) {
	if key != string(model.WebVTT) && key != string(model.TTML) || data.Len() == 0 || a == model.BottomCenter {
		return
	}
	if _, ok := data.Get("alignment"); !ok {
		data.Set("alignment", int64(a.Numpad()))
	}
}

func (r *reader) comments(s *model.Subtitle, root *model.Data, skipped map[int]bool) error {
	v, _ := root.Get("comments")
	list, ok := v.([]any)
	if !ok && v != nil {
		return parseError("The field comments must be a list.")
	}
	skippedCues := slices.Sorted(maps.Keys(skipped))
	var comments []model.Comment
	for i, entry := range list {
		text, before, msg := comment(entry, i)
		if msg != "" {
			if err := r.fail(msg, model.NoBlock, entry); err != nil {
				return err
			}
			continue
		}
		at := int(min(before, math.MaxInt))
		skippedBefore, _ := slices.BinarySearch(skippedCues, at)
		comments = append(comments, model.Comment{Text: text, BeforeCue: at - skippedBefore})
	}
	// AddComment sorts all comments when one comes before the last, so the comments go in sorted.
	slices.SortStableFunc(comments, func(a, b model.Comment) int { return cmp.Compare(a.BeforeCue, b.BeforeCue) })
	for _, c := range comments {
		_ = s.AddComment(c.Text, c.BeforeCue) // the index is 0 or more
	}
	return nil
}

func comment(entry any, i int) (string, int64, string) {
	path := "The field comments[" + strconv.Itoa(i) + "]"
	d, ok := asObject(entry).(*model.Data)
	if !ok {
		return "", 0, path + " must be an object."
	}
	text, ok := d.String("text")
	if !ok {
		return "", 0, path + ".text must be a string."
	}
	v, _ := d.Get("beforeCueIndex")
	before, ok := integer(v)
	if !ok || before < 0 {
		return "", 0, path + ".beforeCueIndex must be an integer of 0 or more."
	}
	return text, before, ""
}
