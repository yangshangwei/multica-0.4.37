package auth

import (
	"reflect"
	"strings"
	"testing"
)

func TestTemporarilyDisabledUsersSnapshotMatchesAuthentication(t *testing.T) {
	ids, emails := TemporarilyDisabledUsers()
	if len(ids) != len(temporarilyDisabledUserIDs) || len(emails) != len(temporarilyDisabledUserEmails) {
		t.Fatal("query snapshot does not contain the authentication denylist")
	}
	for _, id := range ids {
		if !IsTemporarilyDisabledUser("\t "+strings.ToUpper(id)+"\u3000", "") {
			t.Fatal("query ID does not match the case/whitespace-normalized authentication predicate")
		}
	}
	for _, email := range emails {
		if !IsTemporarilyDisabledUser("", "\u0085"+strings.ToUpper(email)+" \n") {
			t.Fatal("query email does not match the case/whitespace-normalized authentication predicate")
		}
	}
	wantIDs, wantEmails := append([]string(nil), ids...), append([]string(nil), emails...)
	if len(ids) > 0 {
		ids[0] = "changed-by-query-caller"
	}
	if len(emails) > 0 {
		emails[0] = "changed-by-query-caller@example.invalid"
	}
	gotIDs, gotEmails := TemporarilyDisabledUsers()
	if !reflect.DeepEqual(gotIDs, wantIDs) || !reflect.DeepEqual(gotEmails, wantEmails) {
		t.Fatal("a query caller changed the process-wide denylist")
	}
}

func TestTemporarilyDisabledUsersEmptySnapshot(t *testing.T) {
	previousIDs, previousEmails := temporarilyDisabledUserIDs, temporarilyDisabledUserEmails
	temporarilyDisabledUserIDs, temporarilyDisabledUserEmails = nil, nil
	t.Cleanup(func() { temporarilyDisabledUserIDs, temporarilyDisabledUserEmails = previousIDs, previousEmails })
	ids, emails := TemporarilyDisabledUsers()
	if ids == nil || emails == nil || len(ids) != 0 || len(emails) != 0 || IsTemporarilyDisabledUser("", "") {
		t.Fatal("an empty emergency denylist must supply empty query filters without denying accounts")
	}
}
