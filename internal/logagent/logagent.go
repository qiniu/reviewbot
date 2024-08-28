package logagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/qiniu/reviewbot/internal/storage"
	"github.com/qiniu/x/log"
)

type LogAgent struct {
	LinterIOMap    map[string]map[string]io.ReadCloser
	mut            sync.Mutex
	IoReaderStatus map[string]*IoReaderStatus
}

type IoReaderStatus string

var Completed IoReaderStatus = "completed"
var Processing IoReaderStatus = "processing"

func NewLogAgent() *LogAgent {
	return &LogAgent{
		LinterIOMap:    make(map[string]map[string]io.ReadCloser),
		mut:            sync.Mutex{},
		IoReaderStatus: make(map[string]*IoReaderStatus),
	}
}

func (*LogAgent) GetArchivedLog(sts []storage.Storage, path string) (string, error) {
	for _, s := range sts {
		url, err := s.Reader(context.Background(), path)
		if err != nil {
			log.Warnf("get storage was filed ,contnue next")
			continue
		}
		return url, nil
	}

	return "", errors.New("cant find at least one url ")
}
func (la *LogAgent) GetLinterLog(linterName, id string) ([]byte, error) {
	if la == nil {
		return []byte{}, errors.New("do not find log agent")
	}
	if la.IoReaderStatus[id] == &Completed {
		return []byte{}, errors.New("the reader was closed")
	}
	var read io.ReadCloser
	la.mut.Lock()
	idMap, ok := la.LinterIOMap[linterName]
	if ok {
		read, ok = idMap[id]
	}
	la.mut.Unlock()
	if !ok {
		return []byte{}, fmt.Errorf("the %s of %s  was not found", id, linterName)
	}
	output, err := io.ReadAll(read)
	// todo 指针重置
	return output, err
}

func (la *LogAgent) UpdateIoMap(linterName, id string, reader io.ReadCloser) error {
	if la.LinterIOMap == nil {
		la.LinterIOMap = make(map[string]map[string]io.ReadCloser)
	}

	if linterName == "" || id == "" {
		return fmt.Errorf("linterName and id must not be empty")
	}

	if la.LinterIOMap[linterName] == nil {
		la.LinterIOMap[linterName] = make(map[string]io.ReadCloser)
	}

	if existingReader, exists := la.LinterIOMap[linterName][id]; exists {
		if err := existingReader.Close(); err != nil {
			return fmt.Errorf("failed to close existing reader: %w", err)
		}
	}

	la.LinterIOMap[linterName][id] = reader

	return nil
}
