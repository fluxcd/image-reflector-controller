/*
Copyright 2021 The Flux authors

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

package policy

import (
	"fmt"
	"regexp"
)

// RegexFilter represents a regular expression filter
type RegexExtractor struct {
	filtered []AttributeExtracted

	Regexp  *regexp.Regexp
	Extract []string
}

type AttributeExtracted struct {
	Tag 		string
	Extracted   string
	Attributes	map[string]string
}

// NewRegexFilter constructs new RegexFilter object
func NewRegexExtractor(pattern string, extract []string) (*RegexExtractor, error) {
	m, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression pattern '%s': %w", pattern, err)
	}
	return &RegexExtractor{
		Regexp:  m,
		Extract: extract,
	}, nil
}

// Apply will construct the filtered list of tags based on the provided list of tags
func (e *RegexExtractor) Apply(list []string) {
	for _, item := range list {
		if submatches := e.Regexp.FindStringSubmatchIndex(item); len(submatches) > 0 {
			var extracted = AttributeExtracted{}
			extracted.Tag = item
			extracted.Attributes = e.extractData(item, submatches)
			e.filtered = append(e.filtered, extracted)
		}
	}
}

// Extract all data group from the regexp
func (e *RegexExtractor) extractData(item string, submatches []int)  map[string]string {
	data := map[string]string{}

	for _, field := range e.Extract {
		result := []byte{}
		result = e.Regexp.ExpandString(result, field, item, submatches)

		key := cleanKey(field)
		data[key] = string(result)
	}

	return data
}

func (e *RegexExtractor) Reduce(discriminator string, sorter string, policer Policer) (map[string]AttributeExtracted, error) {
	distributedAndExtracted := map[string]map[string]AttributeExtracted{}

	cleanDiscriminator := cleanKey(discriminator)
	cleanSorter := cleanKey(sorter)

	for _, tag := range e.filtered {
		division := tag.Attributes[cleanDiscriminator]
		if _, ok := distributedAndExtracted[division]; !ok {
			distributedAndExtracted[division] = map[string]AttributeExtracted{}
		}
		extracted := tag.Attributes[cleanSorter]
		tag.Extracted = extracted
		distributedAndExtracted[division][extracted] = tag
	}

	ret := map[string]AttributeExtracted{}
	for distribution, extracted := range distributedAndExtracted {
		var keys []string
		for k := range extracted {
			keys = append(keys, k)
		}
		latest, err := policer.Latest(keys)

		if err != nil {
			return nil, err
		}
		ret[distribution] = extracted[latest]
	}

	return ret, nil
}

func cleanKey(field string) string {
	key := field
	if field[0] == '$' {
		key = field[1:]
	}
	return key
}