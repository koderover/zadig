/*
Copyright 2025 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// Validation helpers
func validateApplicationBaseFields(app *commonmodels.Application) error {
	if app.Name == "" || app.Key == "" || app.Project == "" {
		return e.ErrInvalidParam.AddDesc("name, key, project are required")
	}
	return nil
}

func validateCustomFields(app *commonmodels.Application) error {
	defs, err := commonmongodb.NewApplicationFieldDefinitionColl().List(context.Background())
	if err != nil {
		return err
	}
	defMap := map[string]*commonmodels.ApplicationFieldDefinition{}
	for _, d := range defs {
		defMap[d.Key] = d
	}

	values := map[string]interface{}{}
	if app.CustomFields != nil {
		values = app.CustomFields
	}

	for key, def := range defMap {
		if def.Required {
			v, ok := values[key]
			if !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("missing required custom field: %s", key))
			}
			if isEmptyByType(def.Type, v) {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("required custom field %s must not be empty", key))
			}
		}
	}

	for key, val := range values {
		def, ok := defMap[key]
		if !ok {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("unknown custom field: %s", key))
		}
		switch def.Type {
		case string(config.ApplicationCustomFieldTypeText), string(config.ApplicationCustomFieldTypeSingleSelect), string(config.ApplicationCustomFieldTypeLink), string(config.ApplicationCustomFieldTypeUser), string(config.ApplicationCustomFieldTypeUserGroup), string(config.ApplicationCustomFieldTypeProject):
			if _, ok := val.(string); !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be string", key))
			}
			if def.Type == string(config.ApplicationCustomFieldTypeSingleSelect) && len(def.Options) > 0 {
				s := val.(string)
				if !contains(def.Options, s) {
					return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s not in options", key))
				}
			}
		case string(config.ApplicationCustomFieldTypeNumber):
			if _, ok := val.(float64); !ok {
				// allow numeric strings to be coerced later? For now enforce JSON number
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be number", key))
			}
		case string(config.ApplicationCustomFieldTypeMultiSelect):
			arr, ok := val.([]interface{})
			if !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be array of strings", key))
			}
			for _, it := range arr {
				s, ok := it.(string)
				if !ok {
					return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be array of strings", key))
				}
				if len(def.Options) > 0 && !contains(def.Options, s) {
					return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s contains value not in options", key))
				}
			}
		case string(config.ApplicationCustomFieldTypeBool):
			if _, ok := val.(bool); !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be bool", key))
			}
			if _, ok := val.(int64); !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be unix timestamp (number)", key))
			}

		default:
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("unsupported custom field type: %s", def.Type))
		}
	}

	return nil
}

// isEmptyByType determines emptiness for required field validation, per field definition type.
func isEmptyByType(defType string, v interface{}) bool {
	switch defType {
	case string(config.ApplicationCustomFieldTypeText), string(config.ApplicationCustomFieldTypeSingleSelect), string(config.ApplicationCustomFieldTypeLink), string(config.ApplicationCustomFieldTypeUser), string(config.ApplicationCustomFieldTypeUserGroup), string(config.ApplicationCustomFieldTypeProject):
		s, ok := v.(string)
		return !ok || strings.TrimSpace(s) == ""
	case string(config.ApplicationCustomFieldTypeNumber), string(config.ApplicationCustomFieldTypeDatetime):
		switch v.(type) {
		case float64, int64:
			return false
		default:
			return true
		}
	case string(config.ApplicationCustomFieldTypeBool):
		_, ok := v.(bool)
		return !ok
	case string(config.ApplicationCustomFieldTypeMultiSelect):
		arr, ok := v.([]interface{})
		if !ok {
			return true
		}
		if len(arr) == 0 {
			return true
		}
		for _, it := range arr {
			s, ok := it.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

func CreateApplication(app *commonmodels.Application, logger *zap.SugaredLogger) (*commonmodels.Application, error) {
	if app == nil {
		return nil, e.ErrInvalidParam.AddDesc("empty body")
	}
	if err := validateApplicationBaseFields(app); err != nil {
		return nil, err
	}
	if err := validateCustomFields(app); err != nil {
		return nil, err
	}
	oid, err := commonmongodb.NewApplicationColl().Create(context.Background(), app)
	if err != nil {
		return nil, err
	}
	app.ID = oid
	return app, nil
}

func GetApplication(id string, logger *zap.SugaredLogger) (*commonmodels.Application, error) {
	return commonmongodb.NewApplicationColl().GetByID(context.Background(), id)
}

func UpdateApplication(id string, app *commonmodels.Application, logger *zap.SugaredLogger) error {
	if app == nil {
		return e.ErrInvalidParam.AddDesc("empty body")
	}
	old, err := commonmongodb.NewApplicationColl().GetByID(context.Background(), id)
	if err != nil {
		return err
	}

	if app.Key != old.Key {
		return e.ErrInvalidParam.AddDesc("key is immutable")
	}

	if err := validateApplicationBaseFields(app); err != nil {
		return err
	}
	if err := validateCustomFields(app); err != nil {
		return err
	}

	app.ID = old.ID
	if err := commonmongodb.NewApplicationColl().UpdateByID(context.Background(), id, app); err != nil {
		return err
	}
	return nil
}

func DeleteApplication(id string, logger *zap.SugaredLogger) error {
	return commonmongodb.NewApplicationColl().DeleteByID(context.Background(), id)
}

// Search with filter list model (re-used from earlier design), with validation unaffected.
type Filter struct {
	Field           string      `json:"field"`
	Verb            string      `json:"verb"`
	Value           interface{} `json:"value"`
	CaseInsensitive *bool       `json:"case_insensitive,omitempty"`
	ExcludeNulls    *bool       `json:"exclude_nulls,omitempty"`
}

type SearchApplicationsRequest struct {
	Page            int64    `json:"page"`
	PageSize        int64    `json:"page_size"`
	Q               string   `json:"q"`
	Filters         []Filter `json:"filters"`
	SortBy          string   `json:"sort_by"`
	SortOrder       string   `json:"sort_order"`
	SortInsensitive bool     `json:"sort_insensitive"`
}

func SearchApplications(req *SearchApplicationsRequest, logger *zap.SugaredLogger) ([]*commonmodels.Application, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	defs, _ := commonmongodb.NewApplicationFieldDefinitionColl().List(context.Background())
	defMap := map[string]*commonmodels.ApplicationFieldDefinition{}
	for _, d := range defs {
		defMap[d.Key] = d
	}
	query := bson.M{}
	ands := make([]bson.M, 0)
	if strings.TrimSpace(req.Q) != "" {
		ands = append(ands, bson.M{"$or": []bson.M{{"name": bson.M{"$regex": req.Q, "$options": "i"}}, {"key": bson.M{"$regex": req.Q, "$options": "i"}}}})
	}
	if len(req.Filters) > 0 {
		exprs, err := buildFilterQuery(req.Filters, defMap)
		if err != nil {
			return nil, 0, err
		}
		ands = append(ands, exprs...)
	}
	if len(ands) > 0 {
		query["$and"] = ands
	}
	order := int32(1)
	if strings.ToLower(req.SortOrder) == "desc" {
		order = -1
	}
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "update_time"
	}
	sort := bson.D{{Key: sortBy, Value: order}}
	list, total, err := commonmongodb.NewApplicationColl().List(context.Background(), &commonmongodb.ApplicationListOptions{Query: query, Sort: sort, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func buildFilterQuery(filters []Filter, defs map[string]*commonmodels.ApplicationFieldDefinition) ([]bson.M, error) {
	out := make([]bson.M, 0, len(filters))
	for _, f := range filters {
		path, fType, err := resolveField(f.Field, defs)
		if err != nil {
			return nil, err
		}
		expr, err := filterToExpr(path, fType, f)
		if err != nil {
			return nil, err
		}
		out = append(out, expr)
	}
	return out, nil
}

func resolveField(field string, defs map[string]*commonmodels.ApplicationFieldDefinition) (string, string, error) {
	if strings.HasPrefix(field, "field.") {
		key := strings.TrimPrefix(field, "field.")
		def, ok := defs[key]
		if !ok {
			return "", "", e.ErrInvalidParam.AddDesc("unknown custom field: " + key)
		}
		var cat string
		switch def.Type {
		case string(config.ApplicationCustomFieldTypeText), string(config.ApplicationCustomFieldTypeSingleSelect), string(config.ApplicationCustomFieldTypeLink), string(config.ApplicationCustomFieldTypeUser), string(config.ApplicationCustomFieldTypeUserGroup), string(config.ApplicationCustomFieldTypeProject):
			cat = string(config.ApplicationFilterFieldTypeString)
		case string(config.ApplicationCustomFieldTypeNumber), string(config.ApplicationCustomFieldTypeDatetime):
			cat = string(config.ApplicationFilterFieldTypeNumber)
		case string(config.ApplicationCustomFieldTypeBool):
			cat = string(config.ApplicationFilterFieldTypeBool)
		case string(config.ApplicationCustomFieldTypeMultiSelect):
			cat = string(config.ApplicationFilterFieldTypeArray)
		default:
			return "", "", e.ErrInvalidParam.AddDesc("unsupported custom field type: " + def.Type)
		}
		return "custom_fields." + key, cat, nil
	}
	switch field {
	case "name", "key", "project", "language", "description", "create_time", "update_time", "testing_service_id", "production_service_id":
		return field, string(config.ApplicationFilterFieldTypeString), nil
	case "repository.codehost_id":
		return field, string(config.ApplicationFilterFieldTypeNumber), nil
	default:
		return "", "", e.ErrInvalidParam.AddDesc("unknown field: " + field)
	}
}

func filterToExpr(path, fType string, f Filter) (bson.M, error) {
	verb := strings.ToLower(f.Verb)
	ci := true
	if f.CaseInsensitive != nil {
		ci = *f.CaseInsensitive
	}
	excludeNulls := true
	if f.ExcludeNulls != nil {
		excludeNulls = *f.ExcludeNulls
	}
	wrapNeg := func(cond bson.M) bson.M {
		if excludeNulls {
			return bson.M{"$and": []bson.M{{path: bson.M{"$ne": nil}}, {path: cond}}}
		}
		return bson.M{path: cond}
	}
	switch fType {
	case string(config.ApplicationFilterFieldTypeNumber):
		switch verb {
		case string(config.ApplicationFilterActionEq):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: v}, nil
		case string(config.ApplicationFilterActionNe):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$ne": v}}, nil
		case string(config.ApplicationFilterActionLt):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$lt": v}}, nil
		case string(config.ApplicationFilterActionLte):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$lte": v}}, nil
		case string(config.ApplicationFilterActionGt):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$gt": v}}, nil
		case string(config.ApplicationFilterActionGte):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$gte": v}}, nil
		default:
			return nil, e.ErrInvalidParam.AddDesc("unsupported number verb: " + verb)
		}
	case string(config.ApplicationFilterFieldTypeArray):
		switch verb {
		case string(config.ApplicationFilterActionContains):
			vals, err := toStringSlice(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$all": vals}}, nil
		case string(config.ApplicationFilterActionNotContains):
			vals, err := toStringSlice(f.Value)
			if err != nil {
				return nil, err
			}
			return wrapNeg(bson.M{"$not": bson.M{"$elemMatch": bson.M{"$in": vals}}}), nil
		case string(config.ApplicationFilterActionHasAnyOf):
			vals, err := toStringSlice(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$in": vals}}, nil
		case string(config.ApplicationFilterActionIsEmpty):
			return bson.M{path: bson.M{"$size": 0}}, nil
		case string(config.ApplicationFilterActionIsNotEmpty):
			return bson.M{path: bson.M{"$not": bson.M{"$size": 0}}}, nil
		default:
			return nil, e.ErrInvalidParam.AddDesc("unsupported array verb: " + verb)
		}
	case string(config.ApplicationFilterFieldTypeBool):
		if verb != string(config.ApplicationFilterActionIs) {
			return nil, e.ErrInvalidParam.AddDesc("unsupported bool verb: " + verb)
		}
		b, ok := f.Value.(bool)
		if !ok {
			return nil, e.ErrInvalidParam.AddDesc("bool filter expects boolean value")
		}
		return bson.M{path: b}, nil
	case string(config.ApplicationFilterFieldTypeString):
		switch verb {
		case string(config.ApplicationFilterActionEq):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			if ci {
				return bson.M{path: bson.M{"$regex": fmt.Sprintf("^%s$", escapeRegex(s)), "$options": "i"}}, nil
			}
			return bson.M{path: s}, nil
		case string(config.ApplicationFilterActionNe):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			if ci {
				return wrapNeg(bson.M{"$not": bson.M{"$regex": fmt.Sprintf("^%s$", escapeRegex(s)), "$options": "i"}}), nil
			}
			return bson.M{path: bson.M{"$ne": s}}, nil
		case string(config.ApplicationFilterActionBeginsWith):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$regex": "^" + escapeRegex(s), "$options": ciOpt(ci)}}, nil
		case string(config.ApplicationFilterActionNotBeginsWith):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			return wrapNeg(bson.M{"$not": bson.M{"$regex": "^" + escapeRegex(s), "$options": ciOpt(ci)}}), nil
		case string(config.ApplicationFilterActionEndsWith):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$regex": escapeRegex(s) + "$", "$options": ciOpt(ci)}}, nil
		case string(config.ApplicationFilterActionNotEndsWith):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			return wrapNeg(bson.M{"$not": bson.M{"$regex": escapeRegex(s) + "$", "$options": ciOpt(ci)}}), nil
		case string(config.ApplicationFilterActionHasAnyOf):
			vals, err := toStringSlice(f.Value)
			if err != nil {
				return nil, err
			}
			if ci {
				ors := make([]bson.M, 0, len(vals))
				for _, v := range vals {
					ors = append(ors, bson.M{path: bson.M{"$regex": fmt.Sprintf("^%s$", escapeRegex(v)), "$options": "i"}})
				}
				return bson.M{"$or": ors}, nil
			}
			return bson.M{path: bson.M{"$in": vals}}, nil
		default:
			return nil, e.ErrInvalidParam.AddDesc("unsupported string verb: " + verb)
		}
	default:
		if path == "testing_service_id" || path == "production_service_id" {
			switch verb {
			case string(config.ApplicationFilterActionEq):
				s, err := toString(f.Value)
				if err != nil {
					return nil, err
				}
				oid, err := primitive.ObjectIDFromHex(s)
				if err != nil {
					return nil, e.ErrInvalidParam.AddDesc("invalid object id")
				}
				return bson.M{path: oid}, nil
			case string(config.ApplicationFilterActionNe):
				s, err := toString(f.Value)
				if err != nil {
					return nil, err
				}
				oid, err := primitive.ObjectIDFromHex(s)
				if err != nil {
					return nil, e.ErrInvalidParam.AddDesc("invalid object id")
				}
				return wrapNeg(bson.M{"$ne": oid}), nil
			case string(config.ApplicationFilterActionHasAnyOf):
				arr, err := toStringSlice(f.Value)
				if err != nil {
					return nil, err
				}
				oids := make([]primitive.ObjectID, 0, len(arr))
				for _, v := range arr {
					oid, err := primitive.ObjectIDFromHex(v)
					if err != nil {
						return nil, e.ErrInvalidParam.AddDesc("invalid object id")
					}
					oids = append(oids, oid)
				}
				return bson.M{path: bson.M{"$in": oids}}, nil
			default:
				return nil, e.ErrInvalidParam.AddDesc("unsupported object id verb: " + verb)
			}
		}
		return nil, e.ErrInvalidParam.AddDesc("unsupported field type")
	}
}

func escapeRegex(in string) string {
	specials := []string{"\\", ".", "*", "+", "?", "|", "(", ")", "[", "]", "{", "}", "^", "$"}
	for _, s := range specials {
		in = strings.ReplaceAll(in, s, "\\"+s)
	}
	return in
}

func ciOpt(ci bool) string {
	if ci {
		return "i"
	}
	return ""
}

func toFloat64(v interface{}) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return 0, e.ErrInvalidParam.AddErr(err)
		}
		return f, nil
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, e.ErrInvalidParam.AddErr(err)
		}
		return f, nil
	default:
		return 0, e.ErrInvalidParam.AddDesc("invalid number")
	}
}

func toString(v interface{}) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	default:
		return "", e.ErrInvalidParam.AddDesc("invalid string")
	}
}

func toStringSlice(v interface{}) ([]string, error) {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, it := range t {
			s, err := toString(it)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return out, nil
	case []string:
		return t, nil
	default:
		return nil, e.ErrInvalidParam.AddDesc("invalid string array")
	}
}
