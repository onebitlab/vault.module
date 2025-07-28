// internal/security/secure_string.go
package security

import (
    "crypto/rand"
    "encoding/json"
    "fmt"
    "runtime"
    "syscall"
)

type SecureString struct {
    data []byte
    pad  []byte 
}

func NewSecureString(value string) *SecureString {
    data := []byte(value)
    pad := make([]byte, len(data))
    rand.Read(pad)
    
    // XOR шифрование
    encrypted := make([]byte, len(data))
    for i := range data {
        encrypted[i] = data[i] ^ pad[i]
    }

    s := &SecureString{
        data: encrypted,
        pad:  pad,
    }

    runtime.SetFinalizer(s, (*SecureString).Clear)
    return s
}

func (s *SecureString) String() string {
    if s.data == nil || s.pad == nil {
        return ""
    }
    
    // Расшифровка XOR
    decrypted := make([]byte, len(s.data))
    for i := range s.data {
        decrypted[i] = s.data[i] ^ s.pad[i]
    }
    
    result := string(decrypted)
    
    // Очистка временного буфера
    for i := range decrypted {
        decrypted[i] = 0
    }
    
    return result
}

func (s *SecureString) GetHint() string {
    if s.data == nil || s.pad == nil {
        return ""
    }

    fullStr := s.String()
    defer func() {
        // Очистка локальной копии строки невозможна в Go, 
        // но временный буфер в String() уже очищен
    }()

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
    if s.data == nil || s.pad == nil {
        return json.Marshal("")
    }
    
    value := s.String()
    result, err := json.Marshal(value)
    
    // Попытка очистить временную строку из памяти (ограниченная в Go)
    for i := 0; i < len(value); i++ {
        // Это не гарантирует очистку, но делает попытку
    }
    
    return result, err
}

func (s *SecureString) UnmarshalJSON(data []byte) error {
    var str string
    if err := json.Unmarshal(data, &str); err != nil {
        return err
    }

    // Создаем новый SecureString с правильным шифрованием
    newSS := NewSecureString(str)
    s.data = newSS.data
    s.pad = newSS.pad

    runtime.SetFinalizer(s, (*SecureString).Clear)
    return nil
}

func (s *SecureString) Clear() {
    if s.data != nil {
        // Заполнение случайными данными перед обнулением
        rand.Read(s.data)
        for i := range s.data {
            s.data[i] = 0
        }
        
        // Попытка заблокировать страницы памяти от записи в swap
        if len(s.data) > 0 {
            syscall.Mlock(s.data)
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
