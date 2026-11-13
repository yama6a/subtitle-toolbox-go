package youtube

import "github.com/yama6a/subtitle-toolbox-go/v4/model"

func (rd *reader) readJSON(_ []byte) (*model.Subtitle, error) {
	return nil, rd.parseError(model.NoLine, "not yet")
}
