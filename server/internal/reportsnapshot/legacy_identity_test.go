package reportsnapshot

import "testing"

func TestInternalSnapshotLegacyIdentity(t *testing.T) {
	s, err := Build(testInput())
	if err != nil {
		t.Fatal(err)
	}
	if s.FactsDigest != "4efaac26aea8ad4ce3994c30ea8d2320e41910efa01ef9c7ecf81d4e0fbe35e7" ||
		s.ResultDigest != "bd9b7e49084b29aaca26d5435419c94b02d91ade6ae7debed5413d9d791e21d0" ||
		s.SnapshotDigest != "6d094ce548d9f020b27cbab4ccca0cef3e6e484ae6ef876a5fa5c3fc4491f9d0" {
		t.Fatal("public projection changed the existing internal seal identity")
	}
}
