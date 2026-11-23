//go:build agentintegration && !windows

package main

import "regexp"

var rk1PriorAnswer = regexp.MustCompile(`\APRIOR_RUN ([0-9a-f-]{36}) RECORD (COMPLETE|INCOMPLETE) RESULT (PRESENT|ABSENT)[ \t]*\r?\nRESULT_TEXT ([^\r\n]*)(?:\r?\n)?\z`)
var rk1SquadAnswer = regexp.MustCompile(`\AMEMBER_RUN ([0-9a-f-]{36})[ \t]*\r?\nMEMBER_CODE (MEMBER-[A-Z0-9]{6})[ \t]*\r?\nWOKEN_BY (\S+) ([0-9a-f-]{36})[ \t]*(?:\r?\n)?\z`)

func rk1PriorAnswerFields(answer string) []string {
	return rk1PriorAnswer.FindStringSubmatch(answer)
}
