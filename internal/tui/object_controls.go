package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) objectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.objectForm
	controls := f.controls()
	count := len(controls)
	move := func(step int) {
		total := count + len(f.buttons())
		f.cursor = (f.cursor + step + total) % total
		if f.cursor < count && !controls[f.cursor].add {
			c := controls[f.cursor]
			f.caret = len([]rune(f.fields[c.field].Values[c.entry]))
		}
	}
	switch msg.String() {
	case "esc":
		m.clearEditor()
		m.err = nil
		return m, nil
	case "tab", "down":
		move(1)
		return m, nil
	case "shift+tab", "up":
		move(-1)
		return m, nil
	case "pgup", "pgdown":
		step := max(1, (m.height-14)/2)
		if msg.String() == "pgup" {
			step = -step
		}
		move(step)
		return m, nil
	case "ctrl+s":
		return m, m.saveObject()
	case "enter":
		if f.readOnly {
			m.clearEditor()
			return m, nil
		}
		if f.cursor == count+1 {
			m.clearEditor()
			m.err = nil
			return m, nil
		}
		if f.cursor == count {
			return m, m.saveObject()
		}
		c := controls[f.cursor]
		field := &f.fields[c.field]
		if c.add {
			field.Values = append(field.Values, "")
			f.caret = 0
		} else if len(field.Choices) > 0 {
			p := &settingSelection{key: "object:" + field.Label, value: field.Values[c.entry], choices: field.Choices, button: -1}
			for i, value := range p.choices {
				if value == p.value {
					p.cursor = i
				}
			}
			m.settingPopup = p
		} else {
			move(1)
		}
		return m, nil
	case "ctrl+d":
		if f.cursor < count {
			m.removeObjectEntry(controls[f.cursor])
		}
		return m, nil
	}
	if f.cursor >= count {
		if msg.String() == "left" || msg.String() == "right" {
			f.cursor = count + (f.cursor-count+1)%len(f.buttons())
		}
		return m, nil
	}
	c := controls[f.cursor]
	if c.add || len(f.fields[c.field].Choices) > 0 || f.readOnly {
		return m, nil
	}
	f.fields[c.field].Values[c.entry], f.caret = editText(f.fields[c.field].Values[c.entry], f.caret, msg)
	return m, nil
}

func (m *Model) removeObjectEntry(c formControl) {
	f := m.objectForm
	field := &f.fields[c.field]
	if c.add || !field.Multi {
		return
	}
	field.Values = append(field.Values[:c.entry], field.Values[c.entry+1:]...)
	f.cursor = min(f.cursor, len(f.controls()))
	f.caret = 0
}

func (m *Model) objectMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	frame := m.objectFrame()
	if msg.Button == tea.MouseButtonRight {
		m.clearEditor()
		m.err = nil
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelUp {
		return m.objectKey(tea.KeyMsg{Type: tea.KeyUp})
	}
	if msg.Button == tea.MouseButtonWheelDown {
		return m.objectKey(tea.KeyMsg{Type: tea.KeyDown})
	}
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if msg.X < frame.x || msg.X >= frame.x+frame.width || msg.Y < frame.y || msg.Y >= frame.y+frame.height {
		return m, nil
	}
	f := m.objectForm
	for _, hit := range frame.buttons {
		if msg.Y == hit.y && msg.X >= hit.x && msg.X < hit.x+hit.width {
			f.cursor = hit.control
			return m.objectKey(tea.KeyMsg{Type: tea.KeyEnter})
		}
	}
	for _, hit := range frame.hits {
		if msg.Y != hit.y || msg.X < hit.x || msg.X >= hit.x+hit.width {
			continue
		}
		f.cursor = hit.control
		c := f.controls()[f.cursor]
		if hit.remove {
			m.removeObjectEntry(c)
			return m, nil
		}
		if c.add || len(f.fields[c.field].Choices) > 0 {
			return m.objectKey(tea.KeyMsg{Type: tea.KeyEnter})
		}
		value := f.fields[c.field].Values[c.entry]
		f.caret = len([]rune(value))
		f.caret = inputCursor(value, f.caret, msg.X-hit.x-1, max(1, hit.width-2), f.fields[c.field].Secret)
		return m, nil
	}
	return m, nil
}

func (m *Model) saveObject() tea.Cmd {
	f := m.objectForm
	if f.readOnly {
		m.clearEditor()
		return nil
	}
	if f.key == "key.create" {
		return m.createKey()
	}
	if strings.HasPrefix(f.key, "account.") {
		return m.saveAccountForm()
	}
	if f.key == "provider.edit" {
		return m.saveProviderForm()
	}
	if f.item != nil {
		return m.savePolicyObject(*f.item, f.fields, false)
	}
	values := []string{}
	for _, field := range f.fields {
		values = append(values, strings.Join(field.Values, ", "))
	}
	if len(values) == 0 {
		m.err = fmt.Errorf("value is required")
		return nil
	}
	value := values[0]
	if f.key == "hs.derp.urls" || f.key == "server.web.trusted_proxies" {
		items := []string{}
		for _, v := range f.fields[0].Values {
			if v = strings.TrimSpace(v); v != "" {
				items = append(items, v)
			}
		}
		encoded, err := json.Marshal(items)
		if err != nil {
			m.err = err
			return nil
		}
		value = string(encoded)
	}
	if strings.HasPrefix(f.key, "dns.") && len(values) > 1 {
		value = values[0] + " = " + values[1]
	}
	if m.resource != nil {
		if m.resource.kind == "node.owner" {
			value = f.fields[0].Values[0]
		}
		m.editValue = value
		return m.saveResource()
	}
	return m.updateSetting(f.key, value)
}
