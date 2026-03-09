package coverage

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

type Coverage struct {
	Instruction *Metric `json:"instruction,omitempty"`
	Branch      *Metric `json:"branch,omitempty"`
	Line        *Metric `json:"line,omitempty"`
	Complexity  *Metric `json:"complexity,omitempty"`
	Method      *Metric `json:"method,omitempty"`
	Class       *Metric `json:"class,omitempty"`
}

type Metric struct {
	Covered int     `json:"covered"`
	Missed  int     `json:"missed"`
	Percent float64 `json:"percent"`
}

type xmlReport struct {
	XMLName  xml.Name     `xml:"report"`
	Counters []xmlCounter `xml:"counter"`
}

type xmlCounter struct {
	XMLName xml.Name `xml:"counter"`
	Type    string   `xml:"type,attr"`
	Missed  int      `xml:"missed,attr"`
	Covered int      `xml:"covered,attr"`
}

func ParseXML(path string) (*Coverage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var report xmlReport
	if err := xml.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse xml: %w", err)
	}

	cov := &Coverage{}

	for _, c := range report.Counters {
		total := c.Covered + c.Missed
		var percent float64
		if total > 0 {
			percent = float64(c.Covered) / float64(total) * 100
		}

		m := &Metric{
			Covered: c.Covered,
			Missed:  c.Missed,
			Percent: percent,
		}

		switch c.Type {
		case "INSTRUCTION":
			cov.Instruction = m
		case "BRANCH":
			cov.Branch = m
		case "LINE":
			cov.Line = m
		case "COMPLEXITY":
			cov.Complexity = m
		case "METHOD":
			cov.Method = m
		case "CLASS":
			cov.Class = m
		}
	}

	return cov, nil
}

func SaveCoverageJSON(cov *Coverage, dir string) error {
	if cov == nil {
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	path := filepath.Join(dir, "coverage.json")
	data, err := json.MarshalIndent(cov, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func LoadCoverageJSON(path string) (*Coverage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cov Coverage
	if err := json.Unmarshal(data, &cov); err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}

	return &cov, nil
}
