package rewardevent

import "testing"

func TestMarshalAndParseLine(t *testing.T) {
	line, err := MarshalLine(Event{
		Account: "sub/fan1.json",
		Domain:  DomainYunbei,
		Yunbei:  &Yunbei{Today: 12, TodayKnown: true, Balance: 345, BalanceKnown: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	event, matched, err := ParseLine("  " + line)
	if err != nil || !matched {
		t.Fatalf("ParseLine() matched=%v err=%v", matched, err)
	}
	if event.Version != Version || event.Account != "sub/fan1.json" || event.Yunbei == nil || event.Yunbei.Today != 12 || event.Yunbei.Balance != 345 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestParseLineIgnoresRegularLogs(t *testing.T) {
	if _, matched, err := ParseLine("[今日收益] 云贝 +12 / 345"); err != nil || matched {
		t.Fatalf("regular log matched=%v err=%v", matched, err)
	}
}

func TestParseLineRejectsInvalidPayload(t *testing.T) {
	for _, line := range []string{
		Prefix + `{}`,
		Prefix + `{"version":1,"account":"cookie.json","domain":"yunbei"}`,
		Prefix + `{"version":2,"account":"cookie.json","domain":"vip","vip":{}}`,
	} {
		if _, matched, err := ParseLine(line); !matched || err == nil {
			t.Fatalf("invalid line matched=%v err=%v: %s", matched, err, line)
		}
	}
}
