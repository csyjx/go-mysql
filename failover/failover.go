package failover

import (
	"fmt"
)

// Failover will do below things after the master down
//  1. Elect a slave which has the most up-to-date data with old master
//  2. Promote the slave to new master
//  3. Change other slaves to the new master
//
// Limitation:
//  1, All slaves must have the same master before, Failover will check using master server id or uuid
//  2, If the failover error, the whole topology may be wrong, we must handle this error manually
//  3, Slaves must have same replication mode, all use GTID or not
//
func Failover(slaves []*Server) ([]*Server, error) {
	// First check slaves use gtid or not
	gtidMode, err := slaves[0].GTIDUsed()
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(slaves); i++ {
		mode, err := slaves[i].GTIDUsed()
		if err != nil {
			return nil, err
		} else if gtidMode != mode {
			return nil, fmt.Errorf("%s use GTID %s, but %s use GTID %s", slaves[0].addr, gtidMode, slaves[i].addr, mode)
		}
	}

	var h Handler

	if gtidMode == GTIDModeOn {
		h = new(GTIDHandler)
	} else {
		h = new(PseudoGTIDHandler)
	}

	// Stop all slave IO_THREAD and wait the relay log done
	for _, slave := range slaves {
		if err = h.WaitRelayLogDone(slave); err != nil {
			return nil, err
		}
	}

	// Find best slave which has the most up-to-data data
	if slaves, err = h.Sort(slaves); err != nil {
		return nil, err
	}

	// Promote the best slave to master
	if err = h.Promote(slaves[0]); err != nil {
		return nil, err
	}

	// Change master
	for i := 1; i < len(slaves); i++ {
		if err = h.ChangeMasterTo(slaves[i], slaves[0]); err != nil {
			return nil, err
		}
	}

	return slaves, nil
}
