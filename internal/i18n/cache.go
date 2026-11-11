package i18n

// NameText caches one formatted name until its input or catalogue changes.
// Views own a separate instance for each phrase; no lookup tables are involved.
type NameText struct {
	messages    *Messages
	name, value string
}

func (c *NameText) Get(m *Messages, name string, format func(string) string) string {
	if c.messages != m || c.name != name {
		c.messages = m
		c.name = name
		c.value = format(name)
	}
	return c.value
}

// NumberText caches a count label between data publications.
type NumberText struct {
	messages *Messages
	n        int
	value    string
}

func (c *NumberText) Get(m *Messages, n int, format func(int) string) string {
	if c.messages != m || c.n != n {
		c.messages = m
		c.n = n
		c.value = format(n)
	}
	return c.value
}
