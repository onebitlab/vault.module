package security

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"runtime"
)



type SecureString struct {
	data []byte
	pad  []byte 
}


func NewSecureString(value string) *SecureString {
	data := []byte(value)
	pad := make([]byte, len(data))
	rand.Read(pad)

	s := &SecureString{
		data: data,
		pad:  pad,
	}

	runtime.SetFinalizer(s, (*SecureString).Clear)
	return s
}

func (s *SecureString) String() string {
	if s.data == nil {
		return ""
	}
	return string(s.data)
}
func (s *SecureString) GetHint() string {
	if s.data == nil {
		return ""
	}

	fullStr := string(s.data)


	var hint string
	if len(fullStr) >= 6 {
		hint = fmt.Sprintf("%s...%s", fullStr[:3], fullStr[len(fullStr)-3:])
	} else if len(fullStr) > 0 {
		hint = fullStr
	} else {
		hint = ""
	}

	return hint
}

func (s *SecureString) MarshalJSON() ([]byte, error) {
	if s.data == nil {
		return json.Marshal("")
	}
	return json.Marshal(string(s.data))
}


func (s *SecureString) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	
	newSS := NewSecureString(str)
	s.data = newSS.data
	s.pad = newSS.pad

	
	runtime.SetFinalizer(s, (*SecureString).Clear)
	return nil
}

func (s *SecureString) Clear() {
	if s.data != nil {
		rand.Read(s.data)
		for i := range s.data {
			s.data[i] = 0
		}
		s.data = nil
	}
	if s.pad != nil {
		rand.Read(s.pad)
		for i := range s.pad {
			s.pad[i] = 0
		}
		s.pad = nil
	}
	runtime.SetFinalizer(s, nil)
}
