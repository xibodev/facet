package toolbox

import (
	"strings"
	"testing"
)

// One message covered three different problems, and for two of them it was
// actively wrong. A request carrying an unknown field is perfectly valid JSON
// that this tool does not accept; a type mismatch is valid JSON with a wrong
// value. Told its JSON is invalid, an agent re-serializes correct JSON, fails
// again, and concludes the module is broken.
//
// That is not hypothetical: an agent sent video_compose {"mock":true}, got
// "invalid request JSON", abandoned Facet, fell back to raw ffmpeg and reported
// a blank video as a success.
func TestDecodeSaysWhatIsActuallyWrong(t *testing.T) {
	var dst struct {
		Input string `json:"input"`
	}

	t.Run("an unknown field names the field and where to look", func(t *testing.T) {
		err := decode([]byte(`{"mock":true}`), &dst)
		if err == nil {
			t.Fatal("an unknown field was accepted")
		}
		msg := err.Error()
		if strings.Contains(msg, "not valid JSON") {
			t.Errorf("valid JSON reported as invalid: %q", msg)
		}
		if !strings.Contains(msg, "mock") {
			t.Errorf("the offending field is not named: %q", msg)
		}
		if !strings.Contains(msg, "describe") {
			t.Errorf("no route to the real schema: %q", msg)
		}
	})

	t.Run("a wrong type says so rather than blaming the JSON", func(t *testing.T) {
		err := decode([]byte(`{"input":123}`), &dst)
		if err == nil {
			t.Fatal("a wrong type was accepted")
		}
		if !strings.Contains(err.Error(), "wrong type") {
			t.Errorf("a type mismatch was not named: %q", err.Error())
		}
	})

	t.Run("genuinely malformed JSON is still reported as such", func(t *testing.T) {
		err := decode([]byte(`{"input":`), &dst)
		if err == nil {
			t.Fatal("malformed JSON was accepted")
		}
		if !strings.Contains(err.Error(), "not valid JSON") {
			t.Errorf("malformed JSON was misdescribed: %q", err.Error())
		}
	})

	t.Run("a valid request still decodes", func(t *testing.T) {
		if err := decode([]byte(`{"input":"a.mp4"}`), &dst); err != nil {
			t.Errorf("a valid request was rejected: %v", err)
		}
		if dst.Input != "a.mp4" {
			t.Errorf("decoded %q", dst.Input)
		}
	})

	t.Run("two objects are still refused", func(t *testing.T) {
		if err := decode([]byte(`{"input":"a"}{"input":"b"}`), &dst); err == nil {
			t.Error("two JSON objects were accepted")
		}
	})
}
