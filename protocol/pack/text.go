package pack

import (
	"scouter.client.qt/protocol/io"
)

// Text type constants
const (
	TextTypeService    = "service"
	TextTypeMethod     = "method"
	TextTypeSQL        = "sql"
	TextTypeObject     = "object"
	TextTypeReferer    = "referer"
	TextTypeUserAgent  = "ua"
	TextTypeError      = "error"
	TextTypeAPICall    = "apicall"
	TextTypeGroup      = "group"
	TextTypeCity       = "city"
	TextTypeLogin      = "login"
	TextTypeDesc       = "desc"
	TextTypeWebHash    = "web"
	TextTypeHashMsg    = "hmsg"
	TextTypeStackTrace = "stackelem"
)

// TextPack represents a text dictionary entry
type TextPack struct {
	TextType string // Type of text (service, method, sql, etc.)
	Hash     int32  // Hash value
	Text     string // Actual text
}

func (p *TextPack) PackType() byte {
	return TEXT
}

func (p *TextPack) Write(out *io.DataOutputX) {
	out.WriteText(p.TextType)
	out.WriteDecimal32(p.Hash)
	out.WriteText(p.Text)
}

// ReadTextPack reads a TextPack from DataInputX
func ReadTextPack(in *io.DataInputX) (*TextPack, error) {
	p := &TextPack{}
	var err error

	if p.TextType, err = in.ReadText(); err != nil {
		return nil, err
	}
	if p.Hash, err = in.ReadDecimal32(); err != nil {
		return nil, err
	}
	if p.Text, err = in.ReadText(); err != nil {
		return nil, err
	}

	return p, nil
}

// TextList is a helper to hold multiple TextPacks
type TextList struct {
	Texts []*TextPack
}

// NewTextList creates a new TextList
func NewTextList() *TextList {
	return &TextList{
		Texts: make([]*TextPack, 0),
	}
}

// Add adds a TextPack to the list
func (l *TextList) Add(t *TextPack) {
	l.Texts = append(l.Texts, t)
}

// AddText creates and adds a TextPack
func (l *TextList) AddText(textType string, hash int32, text string) {
	l.Add(&TextPack{
		TextType: textType,
		Hash:     hash,
		Text:     text,
	})
}

// Size returns the number of texts
func (l *TextList) Size() int {
	return len(l.Texts)
}

// GetByHash finds a text by hash
func (l *TextList) GetByHash(hash int32) *TextPack {
	for _, t := range l.Texts {
		if t.Hash == hash {
			return t
		}
	}
	return nil
}

// ToMap converts to a hash->text map
func (l *TextList) ToMap() map[int32]string {
	m := make(map[int32]string)
	for _, t := range l.Texts {
		m[t.Hash] = t.Text
	}
	return m
}
