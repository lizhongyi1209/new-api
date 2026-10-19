package common

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// Message carries a user-facing English source key and interpolation data.
// The web console can translate the key while API callers receive readable text.
type Message struct {
	Key    string         `json:"key"`
	Params map[string]any `json:"params,omitempty"`
}

func NewMessage(key string, params ...map[string]any) *Message {
	message := &Message{Key: key}
	if len(params) > 0 {
		message.Params = params[0]
	}
	return message
}

func (m *Message) Error() string {
	if len(m.Params) == 0 {
		return m.Key
	}
	pairs := make([]string, 0, len(m.Params)*2)
	for name, value := range m.Params {
		pairs = append(pairs, "{{"+name+"}}", fmt.Sprint(value))
	}
	return strings.NewReplacer(pairs...).Replace(m.Key)
}

func (m *Message) Fields() gin.H {
	fields := gin.H{"message_key": m.Key}
	if len(m.Params) > 0 {
		fields["message_params"] = m.Params
	}
	return fields
}
