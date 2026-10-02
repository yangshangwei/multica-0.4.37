package daemon

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type managementLockTicket struct {
	Version    int     `json:"version"`
	PID        int     `json:"pid"`
	OwnerNonce string  `json:"owner_nonce"`
	HostID     string  `json:"host_id"`
	BootID     string  `json:"boot_id"`
	Ticket     *string `json:"ticket"`
}

// Ticket readers must allow peers to atomically replace and remove their own
// records while the read retains a snapshot of the opened file.
func readManagementLockFile(path string) ([]byte, error) {
	file, err := openManagementLockFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func managementTickets(directory string) ([]managementLockTicket, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	current, err := managementProcessIdentity()
	if err != nil {
		return nil, err
	}
	tickets := make([]managementLockTicket, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || !managedUUID.MatchString(strings.TrimSuffix(name, ".json")) {
			continue
		}
		path := filepath.Join(directory, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > 1024 {
			return nil, errors.New("invalid management lock record")
		}
		if err = managedCheckPermissions(info, true); err != nil {
			return nil, err
		}
		raw, err := readManagementLockFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var ticket managementLockTicket
		if err = json.Unmarshal(raw, &ticket); err != nil {
			return nil, errors.New("invalid management lock JSON")
		}
		if ticket.Version != 1 || ticket.PID <= 0 || uint64(ticket.PID) > math.MaxUint32 || len(ticket.HostID) != 64 || len(ticket.BootID) != 64 || name != ticket.OwnerNonce+".json" {
			return nil, errors.New("invalid management lock owner")
		}
		if ticket.Ticket != nil {
			if _, ok := managedNumber(*ticket.Ticket); !ok {
				return nil, errors.New("invalid management lock ticket")
			}
		}
		if ticket.HostID == current.hostID && (ticket.BootID != current.bootID || managementProcessDefinitelyDead(ticket.PID)) {
			// Each owner uses a fresh UUID filename, so no new owner can be deleted
			// by the stale-owner cleanup of a fixed lock path.
			if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			continue
		}
		tickets = append(tickets, ticket)
	}
	return tickets, nil
}

func withManagementFileLock(parent string, operation func() error) error {
	directory := filepath.Join(parent, ".installation-locks")
	if err := managedEnsureDirectory(directory, true); err != nil {
		return err
	}
	current, err := managementProcessIdentity()
	if err != nil {
		return err
	}
	owner := managementLockTicket{Version: 1, PID: os.Getpid(), OwnerNonce: uuid.NewString(), HostID: current.hostID, BootID: current.bootID}
	path := filepath.Join(directory, owner.OwnerNonce+".json")
	if err := writeManagementJSON(path, owner); err != nil {
		return err
	}
	defer os.Remove(path)
	tickets, err := managementTickets(directory)
	if err != nil {
		return err
	}
	var maximum int64
	for _, other := range tickets {
		if other.Ticket != nil {
			value, _ := managedNumber(*other.Ticket)
			if value > maximum {
				maximum = value
			}
		}
	}
	if maximum == math.MaxInt64 {
		return errors.New("management lock ticket exhausted")
	}
	ticket := strconv.FormatInt(maximum+1, 10)
	owner.Ticket = &ticket
	if err = writeManagementJSON(path, owner); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		tickets, err = managementTickets(directory)
		if err != nil {
			return err
		}
		blocked := false
		for _, other := range tickets {
			if other.OwnerNonce == owner.OwnerNonce {
				continue
			}
			if other.Ticket == nil {
				blocked = true
				break
			}
			value, _ := managedNumber(*other.Ticket)
			if value < maximum+1 || value == maximum+1 && other.OwnerNonce < owner.OwnerNonce {
				blocked = true
				break
			}
		}
		if !blocked {
			return operation()
		}
		if !time.Now().Before(deadline) {
			return errors.New("installation identity has a live lock owner")
		}
		time.Sleep(25 * time.Millisecond)
	}
}
