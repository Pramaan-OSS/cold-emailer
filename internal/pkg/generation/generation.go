package generation

import (
	"github.com/invopop/jsonschema"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

func GenerateSchema[T any]() map[string]any {
	r := jsonschema.Reflector{AllowAdditionalProperties: false, DoNotReference: true, Anonymous: true}
	var v T
	s := r.Reflect(v)
	s.Version = ""
	return pramaan.MustSchema("warmup conversation", s)
}

type GenerationClient struct{ client pramaan.Caller }

func NewClient(client pramaan.Caller) *GenerationClient { return &GenerationClient{client: client} }
