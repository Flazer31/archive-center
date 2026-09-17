package dto

import (
	"encoding/json"
	"errors"
	"io"
)

// ApplyDefaulter is implemented by generated DTO structs that carry
// an ApplyDefaults method. DecodeWithDefaults calls it automatically
// after a successful JSON unmarshal.
type ApplyDefaulter interface {
	ApplyDefaults()
}

// DecodeWithDefaults reads JSON from r into v (which must be a non-nil
// pointer) and then calls ApplyDefaults() if v implements ApplyDefaulter.
//
// The function preserves explicit zero values sent in JSON because
// ApplyDefaults only fills fields that are still nil after unmarshalling.
func DecodeWithDefaults(r io.Reader, v any) error {
	if v == nil {
		return errors.New("decode target must not be nil")
	}
	if err := json.NewDecoder(r).Decode(v); err != nil {
		return err
	}
	if d, ok := v.(ApplyDefaulter); ok {
		d.ApplyDefaults()
	}
	return nil
}
