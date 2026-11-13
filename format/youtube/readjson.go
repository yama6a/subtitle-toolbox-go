package youtube

import (
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func (rd *reader) readJSON(_ []byte) (*model.Subtitle, error) {
	return nil, rd.parseError(model.NoLine, "not yet")
}

func (rd *reader) readSrv3(_, _ *xmlraw.Node) (*model.Subtitle, error) {
	return nil, rd.parseError(model.NoLine, "not yet")
}
