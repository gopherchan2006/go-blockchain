package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func SaveAsJSON(data interface{}, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error converting to JSON: %w", err)
	}

	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing to file %s: %w", filename, err)
	}

	fmt.Printf("Data successfully written to file: %s\n", filename)
	return nil
}
