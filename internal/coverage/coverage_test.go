package coverage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseXML(t *testing.T) {
	tests := []struct {
		name            string
		xmlContent      string
		wantInstrCover  int
		wantInstrMissed int
		wantInstrPct    float64
		wantLineCover   int
		wantLineMissed  int
		wantLinePct     float64
	}{
		{
			name: "full coverage",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<report>
  <counter type="INSTRUCTION" missed="0" covered="100" />
  <counter type="LINE" missed="0" covered="50" />
  <counter type="BRANCH" missed="0" covered="10" />
</report>`,
			wantInstrCover:  100,
			wantInstrMissed: 0,
			wantInstrPct:    100.0,
			wantLineCover:   50,
			wantLineMissed:  0,
			wantLinePct:     100.0,
		},
		{
			name: "partial coverage",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<report>
  <counter type="INSTRUCTION" missed="25" covered="75" />
  <counter type="LINE" missed="10" covered="40" />
  <counter type="BRANCH" missed="2" covered="8" />
</report>`,
			wantInstrCover:  75,
			wantInstrMissed: 25,
			wantInstrPct:    75.0,
			wantLineCover:   40,
			wantLineMissed:  10,
			wantLinePct:     80.0,
		},
		{
			name: "zero coverage",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<report>
  <counter type="INSTRUCTION" missed="100" covered="0" />
  <counter type="LINE" missed="50" covered="0" />
  <counter type="BRANCH" missed="10" covered="0" />
</report>`,
			wantInstrCover:  0,
			wantInstrMissed: 100,
			wantInstrPct:    0.0,
			wantLineCover:   0,
			wantLineMissed:  50,
			wantLinePct:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			xmlPath := filepath.Join(tmpDir, "coverage.xml")
			err := os.WriteFile(xmlPath, []byte(tt.xmlContent), 0644)
			require.NoError(t, err)

			cov, err := ParseXML(xmlPath)
			require.NoError(t, err)

			assert.Equal(t, tt.wantInstrCover, cov.Instruction.Covered)
			assert.Equal(t, tt.wantInstrMissed, cov.Instruction.Missed)
			assert.Equal(t, tt.wantInstrPct, cov.Instruction.Percent)

			assert.Equal(t, tt.wantLineCover, cov.Line.Covered)
			assert.Equal(t, tt.wantLineMissed, cov.Line.Missed)
			assert.Equal(t, tt.wantLinePct, cov.Line.Percent)
		})
	}
}

func TestParseXMLInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	xmlPath := filepath.Join(tmpDir, "coverage.xml")
	err := os.WriteFile(xmlPath, []byte("invalid xml"), 0644)
	require.NoError(t, err)

	_, err = ParseXML(xmlPath)
	assert.Error(t, err)
}

func TestParseXMLNotExist(t *testing.T) {
	_, err := ParseXML("/nonexistent/coverage.xml")
	assert.Error(t, err)
}

func TestSaveCoverageJSON(t *testing.T) {
	cov := &Coverage{
		Instruction: &Metric{Covered: 75, Missed: 25, Percent: 75.0},
		Line:        &Metric{Covered: 40, Missed: 10, Percent: 80.0},
		Branch:      &Metric{Covered: 8, Missed: 2, Percent: 80.0},
	}

	tmpDir := t.TempDir()
	err := SaveCoverageJSON(cov, tmpDir)
	require.NoError(t, err)

	jsonPath := filepath.Join(tmpDir, "coverage.json")
	data, err := os.ReadFile(jsonPath)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"covered": 75`)
	assert.Contains(t, string(data), `"line"`)
}

func TestSaveCoverageJSONNil(t *testing.T) {
	tmpDir := t.TempDir()
	err := SaveCoverageJSON(nil, tmpDir)
	assert.NoError(t, err)
}

func TestLoadCoverageJSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "coverage.json")
	content := `{"instruction":{"covered":75,"missed":25,"percent":75}}`
	err := os.WriteFile(jsonPath, []byte(content), 0644)
	require.NoError(t, err)

	cov, err := LoadCoverageJSON(jsonPath)
	require.NoError(t, err)
	assert.Equal(t, 75, cov.Instruction.Covered)
	assert.Equal(t, 25, cov.Instruction.Missed)
}

func TestLoadCoverageJSONNotExist(t *testing.T) {
	_, err := LoadCoverageJSON("/nonexistent/coverage.json")
	assert.Error(t, err)
}
