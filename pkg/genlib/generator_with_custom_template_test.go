package genlib

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/config"
)

/*
const cardinalityCfg = `
- name: event.id
  cardinality: 250
- name: process.pid
  fuzziness: 10
  range: 100
`
*/

func Test_ParseTemplate(t *testing.T) {
	testCases := []struct {
		template                  []byte
		expectedOrderFields       []string
		expectedTemplateFieldsMap map[string][]byte
		expectedTrailingTemplate  []byte
	}{
		{
			template:                  []byte("no field"),
			expectedOrderFields:       []string{},
			expectedTemplateFieldsMap: map[string][]byte{},
			expectedTrailingTemplate:  []byte("no field"),
		},
		{
			template:                  []byte("{{.aField}}"),
			expectedOrderFields:       []string{"aField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": nil},
			expectedTrailingTemplate:  nil,
		},
		{
			template:                  []byte("{{.aField}} {{.anotherField}}"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": nil, "anotherField": []byte(" ")},
			expectedTrailingTemplate:  nil,
		},
		{
			template:                  []byte("with prefix {{.aField}} {{.anotherField}}"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("with prefix"), "anotherField": []byte(" ")},
			expectedTrailingTemplate:  nil,
		},
		{
			template:                  []byte("{{.aField}} {{.anotherField}} with trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": nil, "anotherField": []byte(" ")},
			expectedTrailingTemplate:  []byte(" with trailing"),
		},
		{
			template:                  []byte("with prefix {{.aField}} {{.anotherField}} and trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("with prefix"), "anotherField": []byte(" ")},
			expectedTrailingTemplate:  []byte(" and trailing"),
		},
		{
			template:                  []byte("{{.aField}} with { in the middle {{.anotherField}}"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": nil, "anotherField": []byte(" with { in the middle ")},
			expectedTrailingTemplate:  nil,
		},
		{
			template:                  []byte("{ with curly brace as prefix {{.aField}} {{.anotherField}}"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{ with curly brace as prefix "), "anotherField": []byte(" ")},
			expectedTrailingTemplate:  nil,
		},
		{
			template:                  []byte("{ with curly brace as prefix {{.aField}} and { in the middle {{.anotherField}}"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{ with curly brace as prefix "), "anotherField": []byte(" and { in the middle ")},
			expectedTrailingTemplate:  nil,
		},
		{
			template:                  []byte("{ with curly brace as prefix {{.aField}} {{.anotherField}} and trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{ with curly brace as prefix "), "anotherField": []byte(" ")},
			expectedTrailingTemplate:  []byte(" and trailing"),
		},
		{
			template:                  []byte("{ with curly brace as prefix {{.aField}} and { in the middle {{.anotherField}} and trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{ with curly brace as prefix "), "anotherField": []byte(" and { in the middle ")},
			expectedTrailingTemplate:  []byte(" and trailing"),
		},
		{
			template:                  []byte("{ with curly brace as prefix {{.aField}} {{.anotherField}} and { curly brace in trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{ with curly brace as prefix "), "anotherField": []byte(" ")},
			expectedTrailingTemplate:  []byte(" and { curly brace in trailing"),
		},
		{
			template:                  []byte("{ with curly brace as prefix {{.aField}} and { in the middle {{.anotherField}} and { curly brace in trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{ with curly brace as prefix "), "anotherField": []byte(" and { in the middle ")},
			expectedTrailingTemplate:  []byte(" and { curly brace in trailing"),
		},
		{
			template:                  []byte("{ with curly brace as prefix {{.aField}} {{.anotherField}} and { curly brace in trailing with again { curly brace in trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{ with curly brace as prefix "), "anotherField": []byte(" ")},
			expectedTrailingTemplate:  []byte(" and { curly brace in trailing with again { curly brace in trailing"),
		},
		{
			template:                  []byte("{ with curly brace as prefix {{.aField}} and { in the middle {{.anotherField}} and { curly brace in trailing with again { curly brace in trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{ with curly brace as prefix "), "anotherField": []byte(" and { in the middle ")},
			expectedTrailingTemplate:  []byte(" and { curly brace in trailing with again { curly brace in trailing"),
		},
		{
			template:                  []byte("{{{.aField}} with curly brace as prefix just before a field {{.anotherField}} and trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{"), "anotherField": []byte(" with curly brace as prefix just before a field ")},
			expectedTrailingTemplate:  []byte(" and trailing"),
		},
		{
			template:                  []byte("{{{.aField}} with curly brace as prefix just before a field and { in the middle {{.anotherField}} and trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{"), "anotherField": []byte(" with curly brace as prefix just before a field and { in the middle ")},
			expectedTrailingTemplate:  []byte(" and trailing"),
		},
		{
			template:                  []byte("{{{.aField}} with curly brace as prefix just before a field {{.anotherField}} and { curly brace in trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{"), "anotherField": []byte(" with curly brace as prefix just before a field ")},
			expectedTrailingTemplate:  []byte(" and { curly brace in trailing"),
		},
		{
			template:                  []byte("{{{.aField}} with curly brace as prefix just before a field and { in the middle {{.anotherField}} and { curly brace in trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{"), "anotherField": []byte(" with curly brace as prefix just before a field and { in the middle ")},
			expectedTrailingTemplate:  []byte(" and { curly brace in trailing"),
		},
		{
			template:                  []byte("{{{.aField}} with curly brace as prefix just before a field {{.anotherField}} and { curly brace in trailing with again { curly brace in trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{"), "anotherField": []byte(" with curly brace as prefix just before a field ")},
			expectedTrailingTemplate:  []byte(" and { curly brace in trailing with again { curly brace in trailing"),
		},
		{
			template:                  []byte("{{{.aField}} with curly brace as prefix just before a field and { in the middle {{.anotherField}} and { curly brace in trailing with again { curly brace in trailing"),
			expectedOrderFields:       []string{"aField", "anotherField"},
			expectedTemplateFieldsMap: map[string][]byte{"aField": []byte("{"), "anotherField": []byte(" with curly brace as prefix just before a field and { in the middle ")},
			expectedTrailingTemplate:  []byte(" and { curly brace in trailing with again { curly brace in trailing"),
		},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("with template: %s", string(testCase.template)), func(t *testing.T) {
			orderedFields, templateFieldsMap, trailingTemplate := parseCustomTemplate(testCase.template)
			if len(orderedFields) != len(testCase.expectedOrderFields) {
				t.Errorf("Expected equal orderedFields")
			}

			for i := range orderedFields {
				if orderedFields[i] != testCase.expectedOrderFields[i] {
					t.Errorf("Expected ordered field at position %d is wrong (expected: `%s`, given: `%s`", i, testCase.expectedOrderFields[i], orderedFields[i])
				}
			}

			if len(templateFieldsMap) != len(testCase.expectedTemplateFieldsMap) {
				t.Errorf("Expected equal templateFieldsMap")
			}

			for k := range templateFieldsMap {
				if _, ok := testCase.expectedTemplateFieldsMap[k]; !ok {
					t.Errorf("Missing expected field `%s` in templateFieldsMap", k)
				}
			}

			if string(trailingTemplate) != string(testCase.expectedTrailingTemplate) {
				t.Errorf("Expected trailing template is wrong (expected: `%s`, given: `%s`", testCase.expectedTrailingTemplate, trailingTemplate)
			}
		})
	}
}

func Test_EmptyCaseWithCustomTemplate(t *testing.T) {
	template, _ := generateCustomTemplateFromField(Config{}, []Field{})
	t.Logf("with template: %s", string(template))
	g, state := makeGeneratorWithCustomTemplate(t, Config{}, []Field{}, template)

	var buf bytes.Buffer

	if err := g.Emit(state, &buf); err != nil {
		t.Fatal(err)
	}

	if len(buf.Bytes()) != 0 {
		t.Errorf("Expected empty bytes")
	}
}

func Test_CardinalityWithCustomTemplate(t *testing.T) {

	test_CardinalityTWithCustomTemplate[string](t, FieldTypeKeyword)
	test_CardinalityTWithCustomTemplate[int](t, FieldTypeInteger)
	test_CardinalityTWithCustomTemplate[float64](t, FieldTypeFloat)
	test_CardinalityTWithCustomTemplate[string](t, FieldTypeGeoPoint)
	test_CardinalityTWithCustomTemplate[string](t, FieldTypeIP)
	test_CardinalityTWithCustomTemplate[string](t, FieldTypeDate)
}

func test_CardinalityTWithCustomTemplate[T any](t *testing.T, ty string) {
	template := []byte(`{"alpha":"{{.alpha}}"}`)
	if ty == FieldTypeInteger || ty == FieldTypeFloat {
		template = []byte(`{"alpha":{{.alpha}}}`)
	}

	fld := Field{
		Name: "alpha",
		Type: ty,
	}

	t.Logf("for type %s, with template: %s", ty, string(template))
	// It's cardinality per mille, so a bit confusing :shrug:
	for cardinality := 1000; cardinality >= 10; cardinality /= 10 {

		// Add the range to get some variety in integers
		tmpl := "- name: alpha\n  cardinality: %d\n  range: 10000"
		yaml := []byte(fmt.Sprintf(tmpl, cardinality))

		cfg, err := config.LoadConfigFromYaml(yaml)
		if err != nil {
			t.Fatal(err)
		}

		g, state := makeGeneratorWithCustomTemplate(t, cfg, []Field{fld}, template)

		vmap := make(map[any]int)

		nSpins := 16384
		for i := 0; i < nSpins; i++ {

			var buf bytes.Buffer
			if err := g.Emit(state, &buf); err != nil {
				t.Fatal(err)
			}

			m := unmarshalJSONT[T](t, buf.Bytes())

			if len(m) != 1 {
				t.Errorf("Expected map size 1, got %d", len(m))
			}

			v, ok := m[fld.Name]

			if !ok {
				t.Errorf("Missing key %v", fld.Name)
			}

			vmap[v] = vmap[v] + 1
		}

		if len(vmap) != 1000/cardinality {
			t.Errorf("Expected cardinality of %d got %d", 1000/cardinality, len(vmap))
		}
	}
}

func Test_FieldBoolWithCustomTemplate(t *testing.T) {
	fld := Field{
		Name: "alpha",
		Type: FieldTypeBool,
	}

	template := []byte(`{"alpha":{{.alpha}}}`)
	t.Logf("with template: %s", string(template))
	// Enough spins, so we can make sure we get at least one true and at least one false
	var cntTrue int
	nSpins := 1024
	for i := 0; i < nSpins; i++ {
		b := testSingleTWithCustomTemplate[bool](t, fld, nil, template)

		if b {
			cntTrue += 1
		}
	}

	if cntTrue == 0 {
		t.Errorf("No true values, really?")
	}

	if cntTrue == nSpins {
		t.Errorf("No false values, really?")
	}
}

func Test_FieldConstKeywordWithCustomTemplate(t *testing.T) {

	fld := Field{
		Name:  "alpha",
		Type:  FieldTypeConstantKeyword,
		Value: "constant_keyword",
	}

	template := []byte(`{"alpha":{{.alpha}}}`)
	t.Logf("with template: %s", string(template))
	b := testSingleTWithCustomTemplate[string](t, fld, nil, template)
	if b != fld.Value {
		t.Errorf("static value not match")
	}
}

func Test_FieldStaticOverrideStringWithCustomTemplate(t *testing.T) {
	fld := Field{
		Name: "alpha",
		Type: FieldTypeKeyword,
	}

	yaml := []byte("- name: alpha\n  value: beta")
	template := []byte(`{"alpha":{{.alpha}}}`)
	t.Logf("with template: %s", string(template))
	b := testSingleTWithCustomTemplate[string](t, fld, yaml, template)
	if b != "beta" {
		t.Errorf("static value not match")
	}
}

func Test_FieldStaticOverrideNumericWithCustomTemplate(t *testing.T) {
	fld := Field{

		Name: "alpha",
		Type: FieldTypeKeyword,
	}

	yaml := []byte("- name: alpha\n  value: 33")
	template := []byte(`{"alpha":{{.alpha}}}`)
	t.Logf("with template: %s", string(template))
	b := testSingleTWithCustomTemplate[float64](t, fld, yaml, template)

	if b != 33.0 {
		t.Errorf("static value not match")
	}
}

func Test_FieldStaticOverrideBoolWithCustomTemplate(t *testing.T) {
	fld := Field{
		Name: "alpha",
		Type: FieldTypeKeyword,
	}

	yaml := []byte("- name: alpha\n  value: true")
	template := []byte(`{"alpha":{{.alpha}}}`)
	t.Logf("with template: %s", string(template))
	b := testSingleTWithCustomTemplate[bool](t, fld, yaml, template)

	if b != true {
		t.Errorf("static value not match")
	}
}

func Test_FieldGeoPointWithCustomTemplate(t *testing.T) {
	fld := Field{
		Name: "alpha",
		Type: FieldTypeGeoPoint,
	}

	template := []byte(`{"alpha":"{{.alpha}}"}`)
	t.Logf("with template: %s", string(template))
	nSpins := 1024
	for i := 0; i < nSpins; i++ {

		b := testSingleTWithCustomTemplate[string](t, fld, nil, template)

		// Expect geo point in form of lat,long
		// where lat is [-90.0..90.0]
		// and long is  [-180.0..180.0]

		s := strings.Split(b, ",")
		if len(s) != 2 {
			t.Fatal("expected comma separated lat,long")
		}

		lat := s[0]
		long := s[1]

		// no whitespace please
		if len(lat) != len(strings.TrimSpace(lat)) {
			t.Errorf("extra whitespace on latitude %s", lat)
		}

		// no whitespace please
		if len(long) != len(strings.TrimSpace(long)) {
			t.Errorf("extra whitespace on longitude %s", long)
		}

		latF, err := strconv.ParseFloat(lat, 64)
		if err != nil {
			t.Errorf("Fail parse latitude as float")
		}
		longF, err := strconv.ParseFloat(long, 64)
		if err != nil {
			t.Errorf("Fail parse longitude as float")
		}

		if latF < -90.0 || latF > 90.0 {
			t.Errorf("latitude out of range %v", latF)
		}

		if longF < -180.0 || longF > 180.0 {
			t.Errorf("longitutde out of range %v", longF)
		}
	}
}

func Test_FieldDateWithCustomTemplate(t *testing.T) {
	fld := Field{
		Name: "alpha",
		Type: FieldTypeDate,
	}

	template := []byte(`{"alpha":"{{.alpha}}"}`)
	t.Logf("with template: %s", string(template))
	nSpins := rand.Intn(1024) + 1
	for i := 0; i < nSpins; i++ {
		now := time.Now()

		b := testSingleTWithCustomTemplate[string](t, fld, nil, template)

		if ts, err := time.Parse(FieldTypeTimeLayout, b); err != nil {
			t.Errorf("Fail parse timestamp %v", err)
		} else {
			// Timestamp should be +- FieldTypeDurationSpan from now within a second of slop
			ts.Add(time.Second * -1)
			ts.Add(time.Second)

			diff := ts.Sub(now)
			if diff < 0 {
				diff = -diff
			}

			if diff >= FieldTypeTimeRange*time.Second {
				t.Errorf("Date generated out of span range %v", diff)
			}
		}
	}
}

func Test_FieldIPWithCustomTemplate(t *testing.T) {
	fld := Field{
		Name: "alpha",
		Type: FieldTypeIP,
	}

	template := []byte(`{"alpha":"{{.alpha}}"}`)
	t.Logf("with template: %s", string(template))
	nSpins := rand.Intn(1024) + 1
	for i := 0; i < nSpins; i++ {

		b := testSingleTWithCustomTemplate[string](t, fld, nil, template)

		if ip := net.ParseIP(b); ip == nil {
			t.Errorf("Fail parse ip %s", b)
		}
	}
}

func Test_FieldFloatsWithCustomTemplate(t *testing.T) {
	_testNumericWithCustomTemplate[float64](t, FieldTypeDouble)
	_testNumericWithCustomTemplate[float32](t, FieldTypeFloat)
	_testNumericWithCustomTemplate[float32](t, FieldTypeHalfFloat)
	_testNumericWithCustomTemplate[float64](t, FieldTypeScaledFloat)

}

func Test_FieldIntegersWithCustomTemplate(t *testing.T) {
	_testNumericWithCustomTemplate[int](t, FieldTypeInteger)
	_testNumericWithCustomTemplate[int64](t, FieldTypeLong)
	_testNumericWithCustomTemplate[uint64](t, FieldTypeUnsignedLong)
}

func _testNumericWithCustomTemplate[T any](t *testing.T, ty string) {
	fld := Field{
		Name: "alpha",
		Type: ty,
	}

	template := []byte(`{"alpha":{{.alpha}}}`)
	t.Logf("with template: %s", string(template))
	nSpins := rand.Intn(1024) + 1
	for i := 0; i < nSpins; i++ {
		testSingleTWithCustomTemplate[T](t, fld, nil, template)
	}
}

func testSingleTWithCustomTemplate[T any](t *testing.T, fld Field, yaml []byte, template []byte) T {
	var err error
	var cfg Config

	if yaml != nil {
		cfg, err = config.LoadConfigFromYaml(yaml)
		if err != nil {
			t.Fatal(err)
		}
	}

	g, state := makeGeneratorWithCustomTemplate(t, cfg, []Field{fld}, template)

	var buf bytes.Buffer

	if err := g.Emit(state, &buf); err != nil {
		t.Fatal(err)
	}

	// BufferWithMutex should now contain an event shaped like {"alpha": "constant_keyword"}
	m := unmarshalJSONT[T](t, buf.Bytes())

	if len(m) != 1 {
		t.Errorf("Expected map size 1, got %d", len(m))
	}

	v, ok := m[fld.Name]

	if !ok {
		t.Errorf("Missing key %v", fld.Name)

	}

	return v
}

func makeGeneratorWithCustomTemplate(t *testing.T, cfg Config, fields Fields, template []byte) (Generator, *GenState) {
	g, err := NewGeneratorWithCustomTemplate(template, cfg, fields)

	if err != nil {
		t.Fatal(err)
	}

	return g, NewGenState()
}
