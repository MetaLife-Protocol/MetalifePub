package restful

import (
	"encoding/json"
	"go.mindeco.de/ssb-refs"
)

type MessageValue struct {
	Previous  *refs.MessageRef `json:"previous"`
	Sequence  int64            `json:"sequence"`
	Author    refs.FeedRef     `json:"author"`
	Timestamp float64          `json:"timestamp"`
	Hash      string           `json:"hash"`
	Content   json.RawMessage  `json:"content"`
	Signature string           `json:"signature"`
}

type DeserializedMessageStu struct {
	Key       string        `json:"key"`
	Value     *MessageValue `json:"value"`
	Timestamp float64       `json:"timestamp"`
}
