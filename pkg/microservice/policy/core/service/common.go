package service

import (
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
)

type Rule struct {
	Verbs            []string                `json:"verbs"`
	Resources        []string                `json:"resources"`
	Kind             string                  `json:"kind"`
	MatchAttributes  []models.MatchAttribute `json:"match_attributes"`
	RelatedResources []string                `json:"related_resources"`
}

const SystemScope = "*"
const PresetScope = ""
