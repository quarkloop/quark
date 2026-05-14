package extraction

import "testing"

func TestValidateOpenExtractionAcceptsFixtureLikeDocuments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record OpenExtraction
	}{
		{name: "receipt", record: fixtureOpenExtraction("receipt", []OpenAmount{{Label: "total", Value: "42.00", Currency: "EUR", CitationIDs: []string{"cite-1"}}})},
		{name: "cv", record: fixtureOpenExtraction("cv", nil)},
		{name: "paper", record: fixtureOpenExtraction("paper", nil)},
		{name: "catalog", record: fixtureOpenExtraction("catalog", nil)},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateOpenExtraction(tt.record); err != nil {
				t.Fatalf("valid %s-like extraction rejected: %v", tt.name, err)
			}
		})
	}
}

func TestValidateOpenExtractionAllowsUncertainPartialFields(t *testing.T) {
	t.Parallel()

	record := fixtureOpenExtraction("certificate", nil)
	record.KeyFacts = append(record.KeyFacts, OpenFact{
		Subject:   "issuer",
		Predicate: "may_be",
		Object:    "unknown authority",
		Uncertain: true,
	})
	record.Dates = append(record.Dates, OpenDate{Label: "expiry", Value: "unclear", Uncertain: true})
	if err := ValidateOpenExtraction(record); err != nil {
		t.Fatalf("uncertain partial fields should be allowed: %v", err)
	}
}

func TestValidateOpenExtractionRejectsUnsupportedHallucinatedRecords(t *testing.T) {
	t.Parallel()

	record := fixtureOpenExtraction("paper", nil)
	record.KeyFacts = append(record.KeyFacts, OpenFact{
		Subject:    "paper",
		Predicate:  "proves",
		Object:     "unsupported claim",
		Confidence: 0.9,
	})
	if err := ValidateOpenExtraction(record); err == nil {
		t.Fatal("expected unsupported fact without citation or uncertainty to fail")
	}
}

func TestValidateOpenExtractionPreservesRawTextReferencesSeparately(t *testing.T) {
	t.Parallel()

	record := fixtureOpenExtraction("catalog", nil)
	record.Sections = append(record.Sections, OpenSection{
		Title:          "Ideas",
		Summary:        "AI app ideas grouped by category.",
		RawReferenceID: "raw-1",
		CitationIDs:    []string{"cite-1"},
	})
	if err := ValidateOpenExtraction(record); err != nil {
		t.Fatalf("raw text reference should validate separately: %v", err)
	}

	record.Sections[0].RawReferenceID = "missing"
	if err := ValidateOpenExtraction(record); err == nil {
		t.Fatal("expected unknown raw text reference to fail")
	}
}

func fixtureOpenExtraction(kind string, amounts []OpenAmount) OpenExtraction {
	return OpenExtraction{
		DocumentTypeGuess:   kind,
		Summary:             "A " + kind + "-like document.",
		SourceTextAvailable: true,
		Citations:           []ExtractedCitation{{ID: "cite-1", SourceURI: kind + ".pdf", TextSpan: "evidence"}},
		RawTextReferences:   []RawTextReference{{ID: "raw-1", SourceURI: kind + ".pdf", Start: 0, End: 20, Preview: "evidence"}},
		Entities: []ExtractedEntity{
			{ID: "doc", Name: kind + " document", Type: "DOCUMENT"},
			{ID: "topic", Name: "topic", Type: "THING"},
		},
		Relations: []ExtractedRelation{{FromID: "doc", ToID: "topic", Relation: "MENTIONS"}},
		KeyFacts: []OpenFact{{
			Subject:     "doc",
			Predicate:   "has_type",
			Object:      kind,
			Confidence:  0.9,
			CitationIDs: []string{"cite-1"},
		}},
		Dates:    []OpenDate{{Label: "date", Value: "2026-05-14", CitationIDs: []string{"cite-1"}}},
		Amounts:  amounts,
		Tables:   []OpenTable{{Title: "table", Headers: []string{"field"}, Rows: [][]string{{"value"}}, CitationIDs: []string{"cite-1"}}},
		Sections: []OpenSection{{Title: "summary", Summary: "section summary", CitationIDs: []string{"cite-1"}}},
	}
}
