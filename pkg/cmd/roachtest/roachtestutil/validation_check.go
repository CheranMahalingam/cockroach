// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.
//

package roachtestutil

import (
	"context"
	gosql "database/sql"
	"fmt"
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/roachprod/logger"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/redact"
)

// CheckReplicaDivergenceOnDB runs a consistency check via the provided DB. It
// ignores transient errors that can result from the implementation of
// crdb_internal.check_consistency, so a nil result does not prove anything.
//
// The consistency check may not get enough time to complete, but will return
// any inconsistencies that it did find before timing out.
func CheckReplicaDivergenceOnDB(ctx context.Context, l *logger.Logger, db *gosql.DB) error {
	// Speed up consistency checks. The test is done, so let's go full throttle.
	_, err := db.ExecContext(ctx, "SET CLUSTER SETTING server.consistency_check.max_rate = '1GB'")
	if err != nil {
		return err
	}

	// EFOS can slow down consistency checks, since we need to take a snapshot for
	// every range sequentially. Disable them.
	_, err = db.ExecContext(ctx,
		"SET CLUSTER SETTING kv.consistency_queue.testing_fast_efos_acquisition.enabled = true")
	if err != nil {
		return err
	}

	// NB: we set a statement_timeout since context cancellation won't work here.
	// We've seen the consistency checks hang indefinitely in some cases.
	// https://github.com/cockroachdb/cockroach/pull/34520
	//
	// TODO(erikgrinaker): avoid result set buffering. We seem to be receiving
	// results in batches of 64 rows, regardless of results_buffer_size or the
	// row size (e.g. with 16 KB ballast per row). Not clear where this buffering
	// is happening or how to disable it.
	started := timeutil.Now()
	rows, err := db.QueryContext(ctx, `
SET statement_timeout = '20m';
SELECT t.range_id, t.start_key_pretty, t.status, t.detail
FROM crdb_internal.check_consistency(false, '', '') as t;`)
	if err != nil {
		// TODO(tbg): the checks can fail for silly reasons like missing gossiped
		// descriptors, etc. -- not worth failing the test for. Ideally this would
		// be rock solid.
		l.Printf("consistency check failed with %v; ignoring", err)
		return nil
	}
	defer rows.Close()

	logEvery := log.Every(time.Minute)
	logEvery.ShouldLog() // don't immediately log

	const maxReport = 10 // max number of inconsistencies to report
	var finalErr error
	var ranges, inconsistent int
	for rows.Next() {
		var rangeID int32
		var prettyKey, status, detail string
		if scanErr := rows.Scan(&rangeID, &prettyKey, &status, &detail); scanErr != nil {
			l.Printf("consistency check failed with %v; ignoring", scanErr)
			return finalErr // return partial finalErr
		}
		// Only detect replica inconsistencies, and ignore MVCC stats mismatches
		// since these can happen in rare cases due to lease requests not respecting
		// latches: https://github.com/cockroachdb/cockroach/issues/93896
		//
		// TODO(erikgrinaker): We should take storage checkpoints for inconsistent
		// ranges as well, up to maxReport. This requires support in
		// check_consistency() such that we take the checkpoints at the same Raft
		// log index across nodes.
		if status == kvpb.CheckConsistencyResponse_RANGE_INCONSISTENT.String() {
			inconsistent++
			msg := fmt.Sprintf("r%d (%s) is inconsistent: %s %s\n", rangeID, prettyKey, status, detail)
			l.Printf(msg)
			if inconsistent <= maxReport {
				finalErr = errors.CombineErrors(finalErr, errors.Newf("%s", redact.SafeString(msg)))
			} else if inconsistent == maxReport+1 {
				finalErr = errors.CombineErrors(finalErr,
					errors.Newf("max number of inconsistencies %d exceeded", maxReport))
			}
		}

		ranges++
		if logEvery.ShouldLog() {
			l.Printf("consistency checked %d ranges (at key %s)", ranges, prettyKey)
		}
	}
	l.Printf("consistency checked %d ranges in %s, found %d inconsistent ranges",
		ranges, timeutil.Since(started).Round(time.Second), inconsistent)

	if err := rows.Err(); err != nil {
		l.Printf("consistency check failed with %v; ignoring", err)
		return finalErr // return partial finalErr
	}
	return finalErr
}

// CheckInvalidDescriptors returns an error if there exists any descriptors in
// the crdb_internal.invalid_objects virtual table.
func CheckInvalidDescriptors(ctx context.Context, db *gosql.DB) error {
	var invalidIDs string
	if err := timeutil.RunWithTimeout(ctx, "descriptor validation", time.Minute, func(ctx context.Context) error {
		// Because crdb_internal.invalid_objects is a virtual table, by default, the
		// query will take a lease on the database sqlDB is connected to and only run
		// the query on the given database. The "" prefix prevents this lease
		// acquisition and allows the query to fetch all descriptors in the cluster.
		rows, err := db.QueryContext(ctx, `SELECT id, obj_name, error FROM "".crdb_internal.invalid_objects`)
		if err != nil {
			return err
		}
		invalidIDs, err = sqlutils.RowsToDataDrivenOutput(rows)
		return err
	}); err != nil {
		return err
	}

	if invalidIDs != "" {
		return errors.Errorf("the following descriptor ids are invalid\n%v", invalidIDs)
	}
	return nil
}
