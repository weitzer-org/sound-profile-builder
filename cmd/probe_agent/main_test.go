package main

import (
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestApplyVariant(t *testing.T) {
	mk := func() *genai.GenerateContentConfig {
		return &genai.GenerateContentConfig{ResponseMIMEType: "application/json", ResponseSchema: &genai.Schema{}, Tools: []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}}}
	}
	c := mk()
	if u, _ := ApplyVariant("noschema", c, "q"); u != "q" || c.ResponseSchema != nil || c.ResponseMIMEType != "" || len(c.Tools) != 1 {
		t.Fatal("noschema must drop only the schema")
	}
	c = mk()
	if u, _ := ApplyVariant("forcesearch", c, "q"); !strings.HasSuffix(u, ForceSearchSuffix) || c.ResponseSchema == nil {
		t.Fatal("forcesearch must keep the schema and append the instruction")
	}
	c = mk()
	if _, _ = ApplyVariant("nosearch", c, "q"); c.Tools != nil || c.ResponseSchema == nil {
		t.Fatal("nosearch must drop only the tools")
	}
	if _, err := ApplyVariant("bogus", mk(), "q"); err == nil {
		t.Fatal("unknown variant accepted")
	}
}

func TestUserPrompt(t *testing.T) {
	if _, err := UserPrompt("6_acoustician", "q", ""); err == nil {
		t.Fatal("acoustician without sonic input must error")
	}
	if p, _ := UserPrompt("1_tone_historian", "q", ""); p != "User Request: q" {
		t.Fatalf("got %q", p)
	}
}

func TestApplyVariantSources(t *testing.T) {
	base := &genai.Schema{Type: genai.TypeObject, Properties: map[string]*genai.Schema{"a": {Type: genai.TypeString}}, Required: []string{"a"}}
	c := &genai.GenerateContentConfig{ResponseSchema: base}
	u, err := ApplyVariant("sources", c, "q")
	if err != nil || !strings.HasSuffix(u, SourcesSuffix) || c.ResponseSchema.Properties["sources"] == nil || len(c.ResponseSchema.Required) != 2 {
		t.Fatalf("sources variant wrong: %v %+v", err, c.ResponseSchema)
	}
	if len(base.Required) != 1 || base.Properties["sources"] != nil {
		t.Fatal("sources variant must not mutate the shared production schema")
	}
}
