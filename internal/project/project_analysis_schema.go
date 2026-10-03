package project

import (
	"encoding/json"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

// Optional insights use null in strict provider output; the parser still accepts
// omitted insights from older reports and independently enforces byte limits.
const projectAnalysisResponseSchemaDocument = `{
 "type":"object","additionalProperties":false,
 "required":["purpose","architecture","components","entry_points","flows","risks","next_steps","engineering_insight"],
 "properties":{
  "purpose":{"$ref":"#/$defs/text"},
  "architecture":{"$ref":"#/$defs/text"},
  "components":{"$ref":"#/$defs/textList"},
  "entry_points":{"$ref":"#/$defs/textList"},
  "flows":{"$ref":"#/$defs/textList"},
  "risks":{"type":"array","maxItems":%[1]d,"items":{"$ref":"#/$defs/risk"}},
  "next_steps":{"$ref":"#/$defs/textList"},
  "engineering_insight":{"$ref":"#/$defs/optionalInsight"}
 },
 "$defs":{
  "text":{"type":"string","minLength":1,"maxLength":%[2]d},
  "textList":{"type":"array","maxItems":%[1]d,"items":{"$ref":"#/$defs/text"}},
  "risk":{
   "type":"object","additionalProperties":false,
   "required":["severity","summary","engineering_insight"],
   "properties":{
    "severity":{"type":"string","enum":["low","medium","high"]},
    "summary":{"$ref":"#/$defs/text"},
    "engineering_insight":{"$ref":"#/$defs/optionalInsight"}
   }
  },
  "optionalInsight":{"anyOf":[{"type":"null"},{"$ref":"#/$defs/insight"}]},
  "insightText":{"type":"string","minLength":1,"maxLength":250},
  "insight":{
   "type":"object","additionalProperties":false,
   "required":["mechanism","why_it_matters_here","tradeoff_or_failure_mode","transferable_lesson"],
   "properties":{
    "mechanism":{"$ref":"#/$defs/insightText"},
    "why_it_matters_here":{"$ref":"#/$defs/insightText"},
    "tradeoff_or_failure_mode":{"$ref":"#/$defs/insightText"},
    "transferable_lesson":{"$ref":"#/$defs/insightText"}
   }
  }
 }
}`

func projectAnalysisResponseSchema() llm.JSONSchema {
	return llm.JSONSchema{Name: "project_analysis_response", Schema: json.RawMessage(fmt.Sprintf(projectAnalysisResponseSchemaDocument, maxProjectAnalysisItems, maxProjectAnalysisItemBytes))}
}
