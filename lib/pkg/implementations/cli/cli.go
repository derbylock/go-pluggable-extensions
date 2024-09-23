package cliplugin

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

type Implementation struct {
	muReader *sync.Mutex
	ch       chan string
	chErr    chan error
}

func NewImplementation() *Implementation {
	muReader := &sync.Mutex{}
	i := &Implementation{muReader: muReader}

	go func() {
		for {
			if err := i.ReadData(); err != nil {
				i.chErr <- err
			}
		}
	}()
	return i
}

func RegisterExtension[T any, D any](i *Implementation, id string, processor ExtensionProcessor[T, D]) {

}

func (i *Implementation) ReadData() error {
	bufReader := bufio.NewReader(os.Stdin)
	s, err := bufReader.ReadString('\n')
	if err != nil {
		return err
	}
}

func (i *Implementation) Start() {

}

type ExtensionProcessor[T any, D any] interface {
	Process(
		ctx context.Context,
		logger slog.Logger,
		input io.Reader,
		dataChannel chan Message[T, D],
	) (io.Writer, error)
}

type ExtensionLogger slog.Logger

type Message[T any, D any] struct {
	Type string
	Data D
}
