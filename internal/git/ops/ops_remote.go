package ops

import (
	"github.com/runbrick/r-git/internal/git/command"
)

// Shared remote ops helpers can go here.

type RemoteOps struct {
	Pusher        *Pusher
	Fetcher       *Fetcher
	Puller        *Puller
	RemoteManager *RemoteManager
}

func NewRemoteOps(runner *command.Runner) *RemoteOps {
	return &RemoteOps{
		Pusher:        NewPusher(runner),
		Fetcher:       NewFetcher(runner),
		Puller:        NewPuller(runner),
		RemoteManager: NewRemoteManager(runner),
	}
}

// Ensure PushNativeResultType matches Java enums
func PushResultToString(t PushNativeResultType) string {
	switch t {
	case PushSuccess:
		return "SUCCESS"
	case PushForcedUpdate:
		return "FORCED_UPDATE"
	case PushNewRef:
		return "NEW_REF"
	case PushRejected:
		return "REJECTED"
	case PushDeleted:
		return "DELETED"
	case PushUpToDate:
		return "UP_TO_DATE"
	default:
		return "ERROR"
	}
}
