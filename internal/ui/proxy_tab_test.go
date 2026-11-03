package ui

import "testing"

// ProxyJump/ProxyCommand live on the Advanced tab, right after the General fields
func TestAddFormProxyFieldsOnAdvancedTab(t *testing.T) {
	m := NewAddForm("", NewStyles(80), 80, 40, "")
	for _, i := range m.getInputsForCurrentTab() {
		if i == proxyJumpInput || i == proxyCommandInput {
			t.Fatal("proxy fields must not be on the General tab")
		}
	}
	m.focused = tagsInput // last General field
	m.handleNavigation("tab")
	if m.currentTab != tabAdvanced || m.focused != proxyJumpInput {
		t.Errorf("after Tags: tab %d focus %d, want Advanced / ProxyJump", m.currentTab, m.focused)
	}
}
