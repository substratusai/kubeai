package v1

import "encoding/json/jsontext"

// SystemOneRequest is the routing envelope of the System One decision API.
// Payload validation belongs to the inference engine. Only the model is needed
// for routing; backend-specific fields are retained without imposing a schema.
type SystemOneRequest struct {
	Model   string         `json:"model"`
	Unknown jsontext.Value `json:",embed"`
}

func (r *SystemOneRequest) GetModel() string  { return r.Model }
func (r *SystemOneRequest) SetModel(m string) { r.Model = m }
