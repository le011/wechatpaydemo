package until

import (
	"encoding/xml"
	"io"
)

type StringMap map[string]string

type xmlMapEntry struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

func (m *StringMap) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		var e xmlMapEntry

		err := d.Decode(&e)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		/*	if e.XMLName.Local == "sign" {
				continue
			}
		*/
		(*m)[e.XMLName.Local] = e.Value
	}
	return nil
}
