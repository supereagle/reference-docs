/*
Copyright 2016 The Kubernetes Authors.

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

package api

import (
	"fmt"
	"sort"
	"strings"

	"errors"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
)

// Definitions indexes open-api definitions
type Definitions struct {
	ByGroupVersionKind map[string]*Definition
	ByKind             map[string]SortDefinitionsByVersion
}

func (d *Definitions) GetAllDefinitions() map[string]*Definition {
	return d.ByGroupVersionKind
}

func (d *Definition) GroupDisplayName() string {
	if len(d.Group) <= 0 || d.Group == "core" {
		return "Core"
	}
	return string(d.Group)
}

func (d *Definitions) GetOtherVersions(this *Definition) []*Definition {
	defs := d.ByKind[this.Name]
	others := []*Definition{}
	for _, def := range defs {
		if def.Version != this.Version {
			others = append(others, def)
		}
	}
	return others
}

// GetByVersionKind looks up a definition using its primary key (version,kind)
func (d *Definitions) GetByVersionKind(group, version, kind string) (*Definition, bool) {
	key := &Definition{Group: ApiGroup(group), Version: ApiVersion(version), Kind: ApiKind(kind)}
	r, f := d.ByGroupVersionKind[key.Key()]
	return r, f
}

// GetByKey looks up a definition from its key (version.kind)
func (d *Definitions) GetByKey(key string) (*Definition, bool) {
	r, f := d.ByGroupVersionKind[key]
	return r, f
}

// IsComplex returns true if the schema is for a complex (non-primitive) defintions
func (d *Definitions) IsComplex(s spec.Schema) bool {
	_, _, k := GetDefinitionVersionKind(s)
	return len(k) > 0
}

func (d *Definitions) GetForSchema(s spec.Schema) (*Definition, bool) {
	g, v, k := GetDefinitionVersionKind(s)
	if len(k) <= 0 {
		return nil, false
	}
	return d.GetByVersionKind(g, v, k)
}

func (d *Definitions) Put(defintion *Definition) {
	d.ByGroupVersionKind[defintion.Key()] = defintion
}

// Initializes the fields for all definitions
func (d *Definitions) InitializeFieldsForAll() {
	for _, definition := range d.GetAllDefinitions() {
		d.InitializeFields(definition)
	}
}

const patchStrategyKey = "x-kubernetes-patch-strategy"
const patchMergeKeyKey = "x-kubernetes-patch-merge-key"
const resourceNameKey = "x-kubernetes-resource"

// Initializes the fields for a definition
func (d *Definitions) InitializeFields(definition *Definition) {
	for fieldName, property := range definition.schema.Properties {
		def := strings.Replace(property.Description, "\n", " ", -1)
		field := &Field{
			Name:        fieldName,
			Type:        GetTypeName(property),
			Description: def,
		}
		if len(property.Extensions) > 0 {
			if ps, f := property.Extensions.GetString(patchStrategyKey); f {
				field.PatchStrategy = ps
			}
			if pmk, f := property.Extensions.GetString(patchMergeKeyKey); f {
				field.PatchMergeKey = pmk
			}
		}

		if fieldDefinition, found := d.GetForSchema(property); found {
			field.Definition = fieldDefinition
		}
		definition.Fields = append(definition.Fields, field)
	}
}

func (d *Definitions) InitializeOtherVersions() {
	for _, definition := range d.GetAllDefinitions() {
		definition.OtherVersions = d.GetOtherVersions(definition)
	}
}

type Definition struct {
	// open-api schema for the definition
	schema spec.Schema
	// Display name of the definition (e.g. Deployment)
	Name      string
	Group     ApiGroup
	ShowGroup bool
	// Api version of the definition (e.g. v1beta1)
	Version ApiVersion
	Kind    ApiKind

	// InToc is true if this definition should appear in the table of contents
	InToc        bool
	IsInlined    bool
	IsOldVersion bool

	FoundInField     bool
	FoundInOperation bool

	// Inline is a list of definitions that should appear inlined with this one in the documentations
	Inline SortDefinitionsByName

	// AppearsIn is a list of definition that this one appears in - e.g. PodSpec in Pod
	AppearsIn SortDefinitionsByName

	OperationCategories []*OperationCategory

	// Fields is a list of fields in this definition
	Fields Fields

	OtherVersions SortDefinitionsByName
	NewerVersions SortDefinitionsByName

	Sample SampleConfig

	FullName string
	Resource string
}

func (d *Definition) GetOperationGroupName() string {
	if strings.ToLower(d.Group.String()) == "rbac" {
		return "RbacAuthorization"
	}
	return strings.Title(d.Group.String())
}

func (d *Definition) Key() string {
	return fmt.Sprintf("%s.%s.%s", d.Group, d.Version, d.Kind)
}

func (d *Definition) MdLink() string {
	if *UseTags {
		return fmt.Sprintf("[%s](#%s-%s)", d.Name, strings.ToLower(d.Name), d.Version)
	}
	return fmt.Sprintf("[%s](#%s-%s-%s)", d.Name, strings.ToLower(d.Name), d.Version, d.Group)

}

func (d *Definition) HrefLink() string {
	if *UseTags {
		return fmt.Sprintf("<a href=\"#%s-%s\">%s</a>", strings.ToLower(d.Name), d.Version, d.Name)
	}
	return fmt.Sprintf("<a href=\"#%s-%s-%s\">%s</a>", strings.ToLower(d.Name), d.Version, d.Group, d.Name)
}

func (d *Definition) VersionLink() string {
	if *UseTags {
		return fmt.Sprintf("<a href=\"#%s-%s\">%s</a>", strings.ToLower(d.Name), d.Version, d.Version)
	}
	return fmt.Sprintf("<a href=\"#%s-%s-%s\">%s</a>", strings.ToLower(d.Name), d.Version, d.Group, d.Version)
}

func (d Definition) Description() string {
	return d.schema.Description
}

func VisitDefinitions(specs []*loads.Document, fn func(definition *Definition)) {
	groups := map[string]string{}
	for _, spec := range specs {
		for name, spec := range spec.Spec().Definitions {
			resource := ""
			if r, found := spec.Extensions.GetString(resourceNameKey); found {
				resource = r
			}

			parts := strings.Split(name, ".")
			if len(parts) < 4 {
				fmt.Printf("Error: Could not find version and type for definition %s.\n", name)
				continue
			}
			var group, version, kind string
			if parts[len(parts)-3] == "api" {
				// e.g. "io.k8s.kubernetes.pkg.api.v1.Pod"
				group = "core"
				version = parts[len(parts)-2]
				kind = parts[len(parts)-1]
				groups[group] = ""
			} else if parts[len(parts)-4] == "apis" {
				// e.g. "io.k8s.kubernetes.pkg.apis.extensions.v1beta1.Deployment"
				group = parts[len(parts)-3]
				version = parts[len(parts)-2]
				kind = parts[len(parts)-1]
				groups[group] = ""
			} else if parts[len(parts)-3] == "util" || parts[len(parts)-3] == "pkg" {
				// e.g. io.k8s.apimachinery.pkg.util.intstr.IntOrString
				// e.g. io.k8s.apimachinery.pkg.runtime.RawExtension
				continue
			} else {
				panic(errors.New(fmt.Sprintf("Could not locate group for %s", name)))
			}

			fn(&Definition{
				schema:    spec,
				Name:      kind,
				Version:   ApiVersion(version),
				Kind:      ApiKind(kind),
				Group:     ApiGroup(group),
				ShowGroup: !*UseTags,
				Resource:  resource,
			})
		}
	}
}

func (d *Definition) GetSamples() []ExampleText {
	r := []ExampleText{}
	for _, p := range GetExampleProviders() {
		r = append(r, ExampleText{
			Tab:  p.GetTab(),
			Type: p.GetSampleType(),
			Text: p.GetSample(d),
		})
	}
	return r
}

func GetDefinitions(specs []*loads.Document) Definitions {
	d := Definitions{
		ByGroupVersionKind: map[string]*Definition{},
		ByKind:             map[string]SortDefinitionsByVersion{},
	}
	VisitDefinitions(specs, func(definition *Definition) {
		d.Put(definition)
	})
	d.InitializeFieldsForAll()
	for _, def := range d.GetAllDefinitions() {
		d.ByKind[def.Name] = append(d.ByKind[def.Name], def)
	}

	// If there are multiple versions for an object.  Mark all by the newest as old
	// Sort the ByKind index in by version with newer versions coming before older versions.
	for _, l := range d.ByKind {
		if len(l) <= 1 {
			continue
		}
		sort.Sort(l)
		// Mark all version as old
		for i, d := range l {
			if i > 0 {
				d.IsOldVersion = true
			}
		}
	}
	d.InitializeOtherVersions()
	d.initAppearsIn()
	d.initInlinedDefinitions()
	return d
}
