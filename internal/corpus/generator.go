// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package corpus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"os"
	"path"
	"strings"
	"time"

	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/config"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib/fields"
	"github.com/spf13/afero"
)

const (
	templateTypeCustom = iota
	templateTypeGoText
)

var ErrNotValidTemplate = errors.New("please, pass --template-type as one of 'placeholder' or 'gotext'")

type Config = config.Config
type Fields = fields.Fields

// timestamp represent a function providing a timestamp.
// It's used to allow replacing the value with a known one during testing.
type timestamp func() int64

func NewGenerator(config Config, fs afero.Fs, location string) (GeneratorCorpus, error) {
	return GeneratorCorpus{
		config:       config,
		fs:           fs,
		templateType: templateTypeCustom,
		location:     location,
		timestamp:    time.Now().Unix,
	}, nil
}

func NewGeneratorWithTemplate(config Config, fs afero.Fs, location, templateType string) (GeneratorCorpus, error) {

	var templateTypeValue int
	if templateType == "placeholder" {
		templateTypeValue = templateTypeCustom
	} else if templateType == "gotext" {
		templateTypeValue = templateTypeGoText
	} else {
		return GeneratorCorpus{}, ErrNotValidTemplate
	}

	return GeneratorCorpus{
		config:       config,
		fs:           fs,
		templateType: templateTypeValue,
		location:     location,
		timestamp:    time.Now().Unix,
	}, nil
}

// TestNewGenerator sets up a GeneratorCorpus configured to be used in testing.
func TestNewGenerator() GeneratorCorpus {
	f, _ := NewGenerator(Config{}, afero.NewMemMapFs(), "testdata")
	f.timestamp = func() int64 { return 1647345675 }
	return f
}

type GeneratorCorpus struct {
	config       Config
	fs           afero.Fs
	location     string
	templateType int
	// timestamp allow overriding value in tests
	timestamp timestamp
}

func (gc GeneratorCorpus) Location() string {
	return gc.location
}

// bulkPayloadFilename computes the bulkPayloadFilename for the corpus to be generated.
// To provide unique names the provided slug is prepended with current timestamp.
func (gc GeneratorCorpus) bulkPayloadFilename(integrationPackage, dataStream, packageVersion string) string {
	slug := integrationPackage + "-" + dataStream + "-" + packageVersion
	filename := fmt.Sprintf("%d-%s.ndjson", gc.timestamp(), sanitizeFilename(slug))
	return filename
}

// bulkPayloadFilenameWithTemplate computes the bulkPayloadFilename for the corpus to be generated.
// To provide unique names the provided slug is prepended with current timestamp.
func (gc GeneratorCorpus) bulkPayloadFilenameWithTemplate(templatePath string) string {
	slug := path.Base(templatePath)
	ext := path.Ext(templatePath)
	slug = slug[0 : len(slug)-len(ext)]
	filename := fmt.Sprintf("%d-%s%s", gc.timestamp(), sanitizeFilename(slug), sanitizeFilename(ext))
	return filename
}

var corpusLocPerm = os.FileMode(0770)
var corpusPerm = os.FileMode(0660)

func (gc GeneratorCorpus) eventsPayloadFromFields(template []byte, fields Fields, totSize uint64, createPayload []byte, f afero.File) error {

	var evgen genlib.Generator
	var err error
	if len(template) == 0 {
		evgen, err = genlib.NewGenerator(gc.config, fields)
	} else {
		if gc.templateType == templateTypeCustom {
			evgen, err = genlib.NewGeneratorWithCustomTemplate(template, gc.config, fields)
		} else if gc.templateType == templateTypeGoText {
			evgen, err = genlib.NewGeneratorWithTextTemplate(template, gc.config, fields)
		} else {
			return ErrNotValidTemplate
		}

	}

	if err != nil {
		return err
	}

	state := genlib.NewGenState()

	var buf *bytes.Buffer
	if len(template) == 0 {
		buf = bytes.NewBuffer(createPayload)
	} else {
		buf = bytes.NewBufferString("")
	}

	var currentSize uint64
	for currentSize < totSize {
		buf.Truncate(len(createPayload))

		if err := evgen.Emit(state, buf); err != nil {
			return err
		}

		buf.WriteByte('\n')

		if _, err = f.Write(buf.Bytes()); err != nil {
			return err
		}

		currentSize += uint64(buf.Len())
	}

	return evgen.Close()
}

// Generate generates a bulk request corpus and persist it to file.
func (gc GeneratorCorpus) Generate(packageRegistryBaseURL, integrationPackage, dataStream, packageVersion, totSize string) (string, error) {
	totSizeInBytes, err := humanize.ParseBytes(totSize)
	if err != nil {
		return "", fmt.Errorf("cannot generate corpus location folder: %v", err)
	}
	if err := gc.fs.MkdirAll(gc.location, corpusLocPerm); err != nil {
		return "", fmt.Errorf("cannot generate corpus location folder: %v", err)
	}

	payloadFilename := path.Join(gc.location, gc.bulkPayloadFilename(integrationPackage, dataStream, packageVersion))
	f, err := gc.fs.OpenFile(payloadFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, corpusPerm)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	flds, err := fields.LoadFields(ctx, packageRegistryBaseURL, integrationPackage, dataStream, packageVersion)
	if err != nil {
		return "", err
	}

	createPayload := []byte(`{ "create" : { "_index": "metrics-` + integrationPackage + `.` + dataStream + `-default" } }` + "\n")

	err = gc.eventsPayloadFromFields(nil, flds, totSizeInBytes, createPayload, f)
	if err != nil {
		return "", err
	}

	if err := f.Close(); err != nil {
		return "", err
	}

	return payloadFilename, err
}

// GenerateWithTemplate generates a template based corpus and persist it to file.
func (gc GeneratorCorpus) GenerateWithTemplate(templatePath, fieldsDefinitionPath, totSize string) (string, error) {
	totSizeInBytes, err := humanize.ParseBytes(totSize)
	if err != nil {
		return "", fmt.Errorf("cannot generate corpus location folder: %v", err)
	}
	if err := gc.fs.MkdirAll(gc.location, corpusLocPerm); err != nil {
		return "", fmt.Errorf("cannot generate corpus location folder: %v", err)
	}

	payloadFilename := path.Join(gc.location, gc.bulkPayloadFilenameWithTemplate(templatePath))
	f, err := gc.fs.OpenFile(payloadFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, corpusPerm)
	if err != nil {
		return "", err
	}

	template, err := os.ReadFile(templatePath)
	if err != nil {
		return "", err
	}

	if len(template) == 0 {
		return "", errors.New("you must provide a non empty template content")
	}

	ctx := context.Background()
	flds, err := fields.LoadFieldsWithTemplate(ctx, fieldsDefinitionPath)
	if err != nil {
		return "", err
	}

	err = gc.eventsPayloadFromFields(template, flds, totSizeInBytes, nil, f)
	if err != nil {
		return "", err
	}

	if err := f.Close(); err != nil {
		return "", err
	}

	return payloadFilename, err
}

// sanitizeFilename takes care of removing dangerous elements from a string so it can be safely
// used as a bulkPayloadFilename.
// NOTE: does not prevent command injection or ensure complete escaping of input
func sanitizeFilename(s string) string {
	s = strings.Replace(s, " ", "-", -1)
	s = strings.Replace(s, ":", "-", -1)
	s = strings.Replace(s, "/", "-", -1)
	s = strings.Replace(s, "\\", "-", -1)
	return s
}
