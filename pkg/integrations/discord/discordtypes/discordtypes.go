package discordtypes

type Webhook struct {
	DID     string            `json:"did"`
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Rewrite []*WebhookRewrite `json:"rewrite,omitempty"`
	Prefix  string            `json:"prefix,omitempty"`
	Suffix  string            `json:"suffix,omitempty"`
}

type WebhookRewrite struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Payload struct {
	Username  string  `json:"username,omitempty"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	Content   string  `json:"content,omitempty"`
	Embeds    []Embed `json:"embeds,omitempty"`
}

type Embed struct {
	Author      *Author `json:"author,omitempty"`
	Title       string  `json:"title,omitempty"`
	URL         string  `json:"url,omitempty"`
	Description string  `json:"description,omitempty"`
	Color       int     `json:"color,omitempty"`
	Fields      []Field `json:"fields,omitempty"`
	Thumbnail   *Image  `json:"thumbnail,omitempty"`
	Image       *Image  `json:"image,omitempty"`
	Footer      *Footer `json:"footer,omitempty"`
}

type Author struct {
	Name    string `json:"name,omitempty"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type Field struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

type Image struct {
	URL string `json:"url,omitempty"`
}

type Footer struct {
	Text    string `json:"text,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}
