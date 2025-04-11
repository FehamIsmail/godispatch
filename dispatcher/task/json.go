package task

import (
	"encoding/json"
)

// ToJSON serializes a Task to JSON
func (t *Task) ToJSON() ([]byte, error) {
	// Ensure all dates are in UTC
	if !t.CreatedAt.IsZero() {
		t.CreatedAt = t.CreatedAt.UTC()
	}

	if !t.UpdatedAt.IsZero() {
		t.UpdatedAt = t.UpdatedAt.UTC()
	}

	if !t.NextRetryAt.IsZero() {
		t.NextRetryAt = t.NextRetryAt.UTC()
	}

	if t.Deadline != nil && !t.Deadline.IsZero() {
		utcDeadline := t.Deadline.UTC()
		t.Deadline = &utcDeadline
	}

	return json.Marshal(t)
}

// TaskFromJSON deserializes a Task from JSON
func TaskFromJSON(data []byte) (*Task, error) {
	var t Task
	err := json.Unmarshal(data, &t)
	if err != nil {
		return nil, err
	}

	// Convert timestamps to UTC if they're not zero
	if !t.CreatedAt.IsZero() {
		t.CreatedAt = t.CreatedAt.UTC()
	}

	if !t.UpdatedAt.IsZero() {
		t.UpdatedAt = t.UpdatedAt.UTC()
	}

	if !t.NextRetryAt.IsZero() {
		t.NextRetryAt = t.NextRetryAt.UTC()
	}

	if t.Deadline != nil && !t.Deadline.IsZero() {
		utcDeadline := t.Deadline.UTC()
		t.Deadline = &utcDeadline
	}

	return &t, nil
}

// marshalJSON is a helper function that marshals an object to JSON
func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// unmarshalJSON is a helper function that unmarshals JSON to an object
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
