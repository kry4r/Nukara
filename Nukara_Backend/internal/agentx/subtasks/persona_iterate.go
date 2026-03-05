package subtasks

import (
	"nukara/backend/internal/agentx/persona"
)

func ParsePersonaPatch(raw string) (persona.Patch, error) {
	patch, err := persona.ParsePatch(raw)
	if err != nil {
		return persona.Patch{}, err
	}
	return persona.ValidatePatch(patch)
}
