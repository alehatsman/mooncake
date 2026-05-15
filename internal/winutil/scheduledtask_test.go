package winutil

import (
	"strings"
	"testing"
	"time"
)

func mooncakeBootTask() Task {
	return Task{
		Name:        "Mooncake-Agentd-Autostart",
		Description: "Start mooncake agentd at boot",
		Triggers: []Trigger{
			{Type: TrigBoot},
		},
		Actions: []ExecAction{
			{
				Command:   `C:\Users\aleh\AppData\Local\Mooncake\bin\mooncake.exe`,
				Arguments: `agentd --bind 0.0.0.0:7879 --token-file "C:\Users\aleh\AppData\Local\Mooncake\agentd.token"`,
			},
		},
		Principal: Principal{
			UserID: `DESKTOP-X\aleh`,
		},
	}
}

func TestTask_Validate_RequiresFields(t *testing.T) {
	cases := []struct {
		name string
		task Task
	}{
		{"empty", Task{}},
		{"missing triggers", Task{Name: "x", Actions: []ExecAction{{Command: "x"}}, Principal: Principal{UserID: "u"}}},
		{"missing actions", Task{Name: "x", Triggers: []Trigger{{Type: TrigBoot}}, Principal: Principal{UserID: "u"}}},
		{"missing principal", Task{Name: "x", Triggers: []Trigger{{Type: TrigBoot}}, Actions: []ExecAction{{Command: "c"}}}},
		{"forbidden char in name", Task{Name: "x|y", Triggers: []Trigger{{Type: TrigBoot}}, Actions: []ExecAction{{Command: "c"}}, Principal: Principal{UserID: "u"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.task.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestTask_Validate_HappyPath(t *testing.T) {
	if err := mooncakeBootTask().Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestTrigger_Validate_RepetitionNeedsInterval(t *testing.T) {
	tr := Trigger{Type: TrigRepetition}
	if err := tr.validate(); err == nil {
		t.Fatal("expected error: missing interval")
	}
}

func TestRenderXML_ContainsExpectedElements(t *testing.T) {
	got, err := RenderXML(mooncakeBootTask())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-16"?>`,
		`<Task version="1.4"`,
		`<URI>\Mooncake-Agentd-Autostart</URI>`,
		`<Description>Start mooncake agentd at boot</Description>`,
		`<BootTrigger>`,
		`<LogonType>S4U</LogonType>`,           // default applied
		`<RunLevel>HighestAvailable</RunLevel>`, // default applied
		`<MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>`, // default applied
		`<Command>C:\Users\aleh\AppData\Local\Mooncake\bin\mooncake.exe</Command>`,
		`<Arguments>agentd --bind 0.0.0.0:7879 --token-file &quot;C:\Users\aleh\AppData\Local\Mooncake\agentd.token&quot;</Arguments>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	// CRLF line endings throughout — matches Export-ScheduledTask output.
	if !strings.Contains(got, "\r\n") {
		t.Errorf("expected CRLF line endings:\n%s", got)
	}
}

func TestRenderXML_RepetitionTrigger(t *testing.T) {
	tk := Task{
		Name: "Keepalive",
		Triggers: []Trigger{{
			Type:               TrigRepetition,
			RepetitionInterval: 5 * time.Minute,
			StartBoundary:      time.Date(2026, 5, 15, 17, 0, 0, 0, time.UTC),
		}},
		Actions:   []ExecAction{{Command: `wsl.exe`, Arguments: `-d Ubuntu-24.04 -u root -- systemctl start ssh`}},
		Principal: Principal{UserID: `X\aleh`},
	}
	got, err := RenderXML(tk)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<TimeTrigger>`,
		`<Interval>PT5M</Interval>`,
		`<StartBoundary>2026-05-15T17:00:00</StartBoundary>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

func TestRenderXML_RestartOnFailure(t *testing.T) {
	tk := mooncakeBootTask()
	tk.Settings.RestartCount = 3
	tk.Settings.RestartInterval = time.Minute
	got, err := RenderXML(tk)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<RestartOnFailure>`,
		`<Interval>PT1M</Interval>`,
		`<Count>3</Count>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

func TestRenderXML_LogonTriggerWithUser(t *testing.T) {
	tk := mooncakeBootTask()
	tk.Triggers = []Trigger{{Type: TrigLogon, LogonUserID: `X\aleh`}}
	got, err := RenderXML(tk)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "<LogonTrigger>") || !strings.Contains(got, `<UserId>X\aleh</UserId>`) {
		t.Errorf("logon trigger XML wrong:\n%s", got)
	}
}

func TestRenderXML_FailsOnInvalid(t *testing.T) {
	_, err := RenderXML(Task{})
	if err == nil {
		t.Fatal("expected error for invalid task")
	}
}

func TestDurationToISO8601(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{0, "PT0S"},
		{5 * time.Minute, "PT5M"},
		{time.Hour, "PT1H"},
		{90 * time.Minute, "PT1H30M"},
		{24 * time.Hour, "P1D"},
		{25 * time.Hour, "P1DT1H"},
		{45 * time.Second, "PT45S"},
		{-1 * time.Second, "PT0S"},
	}
	for _, tc := range cases {
		if got := durationToISO8601(tc.in); got != tc.want {
			t.Errorf("durationToISO8601(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestXMLEscape(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"plain":           "plain",
		"a < b":           "a &lt; b",
		"a > b":           "a &gt; b",
		"a & b":           "a &amp; b",
		`a "quoted" b`:    "a &quot;quoted&quot; b",
		"a 'apo' b":       "a &apos;apo&apos; b",
		"all <>&\"' mix":  "all &lt;&gt;&amp;&quot;&apos; mix",
	}
	for in, want := range cases {
		if got := xmlEscape(in); got != want {
			t.Errorf("xmlEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderRegisterCommand_Shape(t *testing.T) {
	got := RenderRegisterCommand("Mooncake-Agentd-Autostart", `C:\Tmp\task.xml`)
	for _, want := range []string{
		`Register-ScheduledTask`,
		`-TaskName 'Mooncake-Agentd-Autostart'`,
		`-Xml (Get-Content -Raw 'C:\Tmp\task.xml')`,
		`-Force`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("register cmd missing %q:\n%s", want, got)
		}
	}
}

func TestRenderUnregisterCommand_Shape(t *testing.T) {
	got := RenderUnregisterCommand("Foo")
	if !strings.Contains(got, "Unregister-ScheduledTask -TaskName 'Foo'") {
		t.Errorf("got: %s", got)
	}
	if !strings.Contains(got, "-ErrorAction SilentlyContinue") {
		t.Errorf("expected silent-on-missing, got: %s", got)
	}
}

func TestNormaliseTaskXML_StripsWhitespaceAndSortsSettings(t *testing.T) {
	in := "<Task>\n  <Settings>\n" +
		"    <StartWhenAvailable>true</StartWhenAvailable>\n" +
		"    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>\n" +
		"    <AllowStartIfOnBatteries>true</AllowStartIfOnBatteries>\n" +
		"  </Settings>\n" +
		"</Task>\n"
	other := "<Task>\n  <Settings>\n" +
		"    <AllowStartIfOnBatteries>true</AllowStartIfOnBatteries>\n" +
		"    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>\n" +
		"    <StartWhenAvailable>true</StartWhenAvailable>\n" +
		"  </Settings>\n" +
		"</Task>\n"
	a := NormaliseTaskXML(in)
	b := NormaliseTaskXML(other)
	if a != b {
		t.Errorf("settings ordering should not affect normalisation:\na: %s\nb: %s", a, b)
	}
}

func TestNormaliseTaskXML_HandlesCRLF(t *testing.T) {
	lf := "<Task>\n<URI>\\x</URI>\n</Task>\n"
	crlf := "<Task>\r\n<URI>\\x</URI>\r\n</Task>\r\n"
	if NormaliseTaskXML(lf) != NormaliseTaskXML(crlf) {
		t.Errorf("CRLF should normalise to LF")
	}
}

func TestRenderXML_AndNormalise_Roundtrip(t *testing.T) {
	// Render the canonical task, then normalise — the normalised
	// output should still contain all the load-bearing identifiers.
	got, err := RenderXML(mooncakeBootTask())
	if err != nil {
		t.Fatal(err)
	}
	norm := NormaliseTaskXML(got)
	for _, want := range []string{
		`<URI>\Mooncake-Agentd-Autostart</URI>`,
		`<BootTrigger>`,
		`<LogonType>S4U</LogonType>`,
	} {
		if !strings.Contains(norm, want) {
			t.Errorf("normalised xml missing %q:\n%s", want, norm)
		}
	}
}
