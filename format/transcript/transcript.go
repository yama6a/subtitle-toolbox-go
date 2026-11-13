// Package transcript reads and writes the JSON and HTML transcripts of speech-to-text tools and podcasts.
// The format issues add Read, Write and Detect per format.
package transcript

// ReadOptions holds the read settings that the transcript formats share.
type ReadOptions struct {
	// WordTimestamps fills Cue.Words from the word times of the source. False gives cues without words.
	WordTimestamps bool
	// SpeakerVoices sets Cue.Speaker from the speaker of the source. False leaves it empty.
	// The Podcasting 2.0 and HTML transcripts always set it.
	SpeakerVoices bool
	// KeepSegments gives one cue per segment of a Podcasting 2.0 transcript. False joins one-word segments.
	KeepSegments bool
}

// WriteOptions holds the write settings of the transcript formats, one nested struct per format with options.
type WriteOptions struct {
	// Podcast holds the settings of the Podcasting 2.0 transcript writer.
	Podcast PodcastWriteOptions
	// HTML holds the settings of the HTML transcript writer.
	HTML HTMLWriteOptions
}
