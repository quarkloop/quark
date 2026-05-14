package extraction

import (
	"errors"
	"fmt"
	"strings"
)

type OpenExtraction struct {
	DocumentTypeGuess   string
	Summary             string
	SourceTextAvailable bool
	KeyFacts            []OpenFact
	Entities            []ExtractedEntity
	Relations           []ExtractedRelation
	Dates               []OpenDate
	Amounts             []OpenAmount
	Tables              []OpenTable
	Sections            []OpenSection
	Citations           []ExtractedCitation
	RawTextReferences   []RawTextReference
	Uncertainties       []string
}

type OpenFact struct {
	Subject     string
	Predicate   string
	Object      string
	Confidence  float32
	CitationIDs []string
	Uncertain   bool
}

type OpenDate struct {
	Label       string
	Value       string
	CitationIDs []string
	Uncertain   bool
}

type OpenAmount struct {
	Label       string
	Value       string
	Currency    string
	CitationIDs []string
	Uncertain   bool
}

type OpenTable struct {
	Title       string
	Headers     []string
	Rows        [][]string
	CitationIDs []string
	Uncertain   bool
}

type OpenSection struct {
	Title          string
	Summary        string
	RawReferenceID string
	CitationIDs    []string
	Uncertain      bool
}

type RawTextReference struct {
	ID        string
	SourceURI string
	Start     int
	End       int
	Preview   string
}

func ValidateOpenExtraction(record OpenExtraction) error {
	if strings.TrimSpace(record.DocumentTypeGuess) == "" {
		return errors.New("document_type_guess is required")
	}
	if strings.TrimSpace(record.Summary) == "" {
		return errors.New("summary is required")
	}
	citationIDs := make(map[string]struct{}, len(record.Citations))
	for _, citation := range record.Citations {
		id := strings.TrimSpace(citation.ID)
		if id == "" {
			return errors.New("citation id is required")
		}
		if strings.TrimSpace(citation.SourceURI) == "" && strings.TrimSpace(citation.TextSpan) == "" {
			return fmt.Errorf("citation %q requires source_uri or text_span", id)
		}
		citationIDs[id] = struct{}{}
	}
	rawIDs := make(map[string]struct{}, len(record.RawTextReferences))
	for _, ref := range record.RawTextReferences {
		id := strings.TrimSpace(ref.ID)
		if id == "" {
			return errors.New("raw text reference id is required")
		}
		if ref.End > 0 && ref.Start > ref.End {
			return fmt.Errorf("raw text reference %q has invalid offsets", id)
		}
		rawIDs[id] = struct{}{}
	}
	if err := ValidateExtractedRecord(ExtractedRecord{
		ProfileID:           "generic-open",
		SourceTextAvailable: record.SourceTextAvailable,
		Facts:               extractedFactsForValidation(record.KeyFacts),
		Entities:            record.Entities,
		Relations:           record.Relations,
		Citations:           record.Citations,
	}); err != nil {
		return err
	}
	for _, fact := range record.KeyFacts {
		if !validOpenConfidence(fact.Confidence) {
			return errors.New("fact confidence must be between 0 and 1")
		}
		if err := validateEvidence("fact", fact.Subject+"/"+fact.Predicate, fact.CitationIDs, fact.Uncertain, record.SourceTextAvailable, citationIDs); err != nil {
			return err
		}
	}
	for _, date := range record.Dates {
		if strings.TrimSpace(date.Label) == "" && strings.TrimSpace(date.Value) == "" {
			continue
		}
		if err := validateEvidence("date", firstNonBlank(date.Label, date.Value), date.CitationIDs, date.Uncertain, record.SourceTextAvailable, citationIDs); err != nil {
			return err
		}
	}
	for _, amount := range record.Amounts {
		if strings.TrimSpace(amount.Label) == "" && strings.TrimSpace(amount.Value) == "" {
			continue
		}
		if err := validateEvidence("amount", firstNonBlank(amount.Label, amount.Value), amount.CitationIDs, amount.Uncertain, record.SourceTextAvailable, citationIDs); err != nil {
			return err
		}
	}
	for _, table := range record.Tables {
		if strings.TrimSpace(table.Title) == "" && len(table.Headers) == 0 && len(table.Rows) == 0 {
			continue
		}
		if err := validateEvidence("table", table.Title, table.CitationIDs, table.Uncertain, record.SourceTextAvailable, citationIDs); err != nil {
			return err
		}
	}
	for _, section := range record.Sections {
		if strings.TrimSpace(section.RawReferenceID) != "" {
			if _, ok := rawIDs[strings.TrimSpace(section.RawReferenceID)]; !ok {
				return fmt.Errorf("section %q references unknown raw text reference %q", section.Title, section.RawReferenceID)
			}
		}
		if strings.TrimSpace(section.Title) == "" && strings.TrimSpace(section.Summary) == "" {
			continue
		}
		if err := validateEvidence("section", firstNonBlank(section.Title, section.Summary), section.CitationIDs, section.Uncertain, record.SourceTextAvailable, citationIDs); err != nil {
			return err
		}
	}
	return nil
}

func extractedFactsForValidation(facts []OpenFact) []ExtractedFact {
	out := make([]ExtractedFact, 0, len(facts))
	for _, fact := range facts {
		if fact.Uncertain {
			continue
		}
		out = append(out, ExtractedFact{
			Subject:     fact.Subject,
			Predicate:   fact.Predicate,
			Object:      fact.Object,
			CitationIDs: fact.CitationIDs,
		})
	}
	return out
}

func validateEvidence(kind, label string, citationIDs []string, uncertain, sourceTextAvailable bool, known map[string]struct{}) error {
	if !sourceTextAvailable || uncertain {
		return nil
	}
	if len(citationIDs) == 0 {
		return fmt.Errorf("%s %q requires citation evidence or uncertainty", kind, label)
	}
	for _, citationID := range citationIDs {
		if _, ok := known[strings.TrimSpace(citationID)]; !ok {
			return fmt.Errorf("%s %q references unknown citation %q", kind, label, citationID)
		}
	}
	return nil
}

func validOpenConfidence(value float32) bool {
	return value >= 0 && value <= 1
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
