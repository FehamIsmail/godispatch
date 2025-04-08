package task

import (
	"encoding/json"
)

// ToJSON serializes a Task to JSON
func (t *Task) ToJSON() ([]byte, error) {
	return marshalJSON(t)
}

// TaskFromJSON deserializes a Task from JSON
func TaskFromJSON(data []byte) (*Task, error) {
	var task Task
	if err := unmarshalJSON(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// marshalJSON is a helper function to serialize to JSON
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// unmarshalJSON is a helper function to deserialize from JSON
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
