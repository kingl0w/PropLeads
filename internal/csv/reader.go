package csv

import (
	"encoding/csv"
	"os"
)

func ReadPIDs(filename string) ([]string, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }

    var pids []string
    for _, record := range records {
        if len(record) > 0 {
            pids = append(pids, record[0])
        }
    }

    return pids, nil
}