package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
)

type banData struct {
	items      []*banitem
	serialbans map[string][]*banitem
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
}

type banitemjson struct {
	ID     int           `json:"id"`
	Values []interface{} `json:"values"`
}

type banjson struct {
	Metadata interface{}   `json:"-"`
	Data     []banitemjson `json:"data"`
}

func getBanData() (*banData, error) {
	req, err := http.NewRequest("GET", "***REMOVED***", nil)
	if err != nil {
		fmt.Println("could not create request", err.Error())
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+os.Getenv("MTABOT_BASIC_AUTH"))

	fmt.Println("downloading")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("could not request data", err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	fmt.Println("downloaded")

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("couldn't read response")
		return nil, err
	}
	fmt.Println("read all")

	data := &banjson{}
	if err := json.Unmarshal(respBytes, data); err != nil {
		fmt.Println("could not unmarshal")
		return nil, err
	}

	fmt.Println("unmarshaled")

	var items []*banitem
	serialbans := make(map[string][]*banitem)
	for _, row := range data.Data {
		rowData := banitemFromInterface(row.Values)
		if rowData != nil {
			items = append(items, rowData)
			serialbans[rowData.Serial] = append(serialbans[rowData.Serial], rowData)
		}
	}

	return &banData{items, serialbans}, nil
}

func banitemFromInterface(data []interface{}) *banitem {
	if len(data) != 11 {
		fmt.Printf("data is bad length %#v\n", data)
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
			t, err = time.Parse("2006-01-02 15:04:05", str)
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
				t, err = time.Parse("2006-01-02 15:04:05", str)
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
	if i.Enabled {
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

	e := &discordgo.MessageEmbed{
		Title: i.Serial,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Note", Value: i.Note},
			{Name: "End date", Value: endDate},
			{Name: "Status", Value: status},
			{Name: "Creation date", Value: i.CreatedAt.Format(time.RFC1123Z)},
		},
	}

	if i.UpdatedAt != nil {
		e.Fields = append(e.Fields, &discordgo.MessageEmbedField{
			Name: "Updated at", Value: i.UpdatedAt.Format(time.RFC1123Z),
		})
	}

	return e
}
