package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

type banData struct {
	items      []*banitem
	serialbans map[string][]*banitem
	repids     map[int]*banitem
}

type banitem struct {
	ID             int        `json:"id"`
	Serial         string     `json:"serial"`
	Reason         string     `json:"type"`
	CreatedAt      *time.Time `json:"added"`
	UpdatedAt      *time.Time `json:"updated"`
	Note           string     `json:"note"`
	Enabled        bool       `json:"enabled"`
	HasEnd         bool       `json:"has_end"`
	ExpiredAt      *time.Time `json:"endtime"`
	AllowedServers string     `json:"allowed_servers"`

	Archived bool `json:"-"`
}

func (i banitem) HasExpired() bool {
	return i.ExpiredAt != nil && i.ExpiredAt.Before(time.Now())
}

func (i banitem) IsActive() bool {
	// Enabled and not expired
	return i.Enabled && !i.HasExpired()
}

type banitemjson struct {
	ID     int           `json:"id"`
	Values []interface{} `json:"values"`
}

type banjson struct {
	Metadata interface{}   `json:"-"`
	Data     []banitemjson `json:"data"`
}

func (result *banData) importFromURL(url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("could not create request", err.Error())
		return err
	}
	req.Header.Set("Authorization", "Basic "+os.Getenv("MTABOT_BASIC_AUTH"))

	fmt.Println("downloading")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("could not request data", err.Error())
		return err
	}
	defer resp.Body.Close()
	fmt.Println("downloaded")

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("couldn't read response")
		return err
	}
	fmt.Println("read all")

	data := &banjson{}
	if err := json.Unmarshal(respBytes, data); err != nil {
		fmt.Println("could not unmarshal")
		return err
	}

	fmt.Println("unmarshaled")

	for _, row := range data.Data {
		rowData := banitemFromInterface(row.Values)
		if rowData != nil {
			result.items = append(result.items, rowData)
			result.serialbans[rowData.Serial] = append(result.serialbans[rowData.Serial], rowData)
			result.repids[rowData.ID] = rowData
		}
	}
	return nil
}

func getBanData() (*banData, error) {
	result := &banData{nil, make(map[string][]*banitem), make(map[int]*banitem)}
	if err := result.importFromURL(os.Getenv("MTABOT_GLOBAL_BANS_ARCHIVE")); err != nil {
		return nil, errors.Wrap(err, "archive")
	}
	for _, row := range result.items {
		row.Archived = true
	}
	if err := result.importFromURL(os.Getenv("MTABOT_GLOBAL_BANS")); err != nil {
		return nil, errors.Wrap(err, "non-archive")
	}
	return result, nil
}

func banitemFromInterface(data []interface{}) *banitem {
	if len(data) != 10 {
		fmt.Printf("data is bad length %d - %#v\n", len(data), data)
		return nil
	}

	item := banitem{
		ID:             int(data[0].(float64)),
		Serial:         data[1].(string),
		Reason:         data[2].(string),
		CreatedAt:      nil, // 3 - parsed below
		UpdatedAt:      nil, // 4 - parsed below
		Note:           data[5].(string),
		Enabled:        data[6].(float64) == 1,
		HasEnd:         data[7].(float64) == 1,
		ExpiredAt:      nil, // 8 - parsed below
		AllowedServers: data[9].(string),
	}

	// Parsing times
	// example: `2014-09-15 07:26:59`
	{
		t, err := time.Parse("2006-01-02 15:04:05", data[3].(string))
		if err != nil {
			fmt.Printf("could not parse created_at, %#v\n", data[3])
			return nil
		}
		item.CreatedAt = &t

		if str := data[4].(string); str != "" {
			t, err := time.Parse("2006-01-02 15:04:05", str)
			if err != nil {
				fmt.Printf("could not parse updated_at, %#v\n", data[4])
				return nil
			}
			item.UpdatedAt = &t
		}

		if str, ok := data[8].(string); ok && str != "" {
			if str == "0000-00-00 00:00:00" {
				item.ExpiredAt = &time.Time{}
			} else {
				t, err := time.Parse("2006-01-02 15:04:05", str)
				if err != nil {
					fmt.Printf("could not parse expired_at, %#v\n", data[8])
					item.ExpiredAt = &time.Time{}
				} else {
					item.ExpiredAt = &t
				}
			}
		}
	}

	return &item
}

func (i *banitem) toEmbed() *discordgo.MessageEmbed {
	status := "Disabled"
	if i.HasExpired() {
		status = "Expired"
		if !i.Enabled {
			status = "~~Expired~~ Disabled"
		}
	} else if i.Enabled {
		status = "Enabled"
	}

	endDate := "Permanent"
	if i.HasEnd {
		if i.ExpiredAt == nil {
			endDate = "Not sure"
		} else {
			endDate = i.ExpiredAt.Format(time.RFC1123Z)
		}
	}

	note := i.Note
	if strings.Contains(note, "[auto]") {
		note = "[auto] Ask anti-cheat team for details"
	}
	if note == "" {
		note = "(none provided)"
	}
	reason := i.Reason
	if reason == "" {
		reason = "(none provided)"
	}

	e := &discordgo.MessageEmbed{
		Description: "[Avoid sharing private information with users](https://discordapp.com/channels/278474088903606273/307874986721542144/677102248848785418)",
		Title:       fmt.Sprintf("%d: %s", i.ID, i.Serial),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Note (**private**)", Value: note},
			{Name: "Reason", Value: reason, Inline: true},
			{Name: "Status", Value: status, Inline: true},
			{Name: "Created at", Value: i.CreatedAt.Format(time.RFC1123Z)},
		},
	}

	if i.UpdatedAt != nil {
		e.Fields = append(e.Fields, &discordgo.MessageEmbedField{
			Name: "Updated at", Value: i.UpdatedAt.Format(time.RFC1123Z),
		})
	}

	e.Fields = append(e.Fields, &discordgo.MessageEmbedField{Name: "Expires at", Value: endDate})

	if i.IsActive() {
		e.Color = 0xff0000
	} else {
		e.Color = 0x00ff00
	}

	return e
}
