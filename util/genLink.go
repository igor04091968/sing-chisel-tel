package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log" // Added for logging
	"net/url"
	"strings"

	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/igor04091968/sing-chisel-tel/util/common"
)

var InboundTypeWithLink = []string{"socks", "http", "mixed", "shadowsocks", "naive", "hysteria", "hysteria2", "anytls", "tuic", "vless", "trojan", "vmess"}

func LinkGenerator(clientConfig json.RawMessage, i *model.Inbound, hostname string) []string {
	inboundMap, err := i.MarshalFull()
	if err != nil {
		log.Printf("LinkGenerator: failed to marshal inbound to map: %v", err)
		return []string{}
	}

	var tls map[string]interface{}
	if i.TlsId > 0 && i.Tls != nil {
		tls = prepareTls(i.Tls)
	}

	var userConfig map[string]map[string]interface{}
	if err := json.Unmarshal(clientConfig, &userConfig); err != nil {
		log.Printf("LinkGenerator: failed to unmarshal client config: %v", err)
		return []string{}
	}

	var Addrs []map[string]interface{}
	if len(i.Addrs) > 0 {
		if err := json.Unmarshal(i.Addrs, &Addrs); err != nil {
			log.Printf("LinkGenerator: failed to unmarshal inbound addrs: %v", err)
			return []string{}
		}
	}
	
	if len(Addrs) == 0 {
		listenPort, ok := (*inboundMap)["listen_port"].(float64)
		if !ok {
			log.Printf("LinkGenerator: listen_port not found or not a number in inbound %s", i.Tag)
			return []string{}
		}
		Addrs = append(Addrs, map[string]interface{}{
			"server":      hostname,
			"server_port": listenPort,
			"remark":      i.Tag,
		})
		if i.TlsId > 0 && tls != nil {
			Addrs[0]["tls"] = tls
		}
	} else {
		for index, addr := range Addrs {
			addrRemark, _ := addr["remark"].(string) // _ is ok here, default to empty string
			Addrs[index]["remark"] = i.Tag + addrRemark
			if i.TlsId > 0 && tls != nil {
				newTls := map[string]interface{}{}
				for k, v := range tls {
					newTls[k] = v
				}

				// Override tls
				if addrTls, ok := addr["tls"].(map[string]interface{}); ok {
					for k, v := range addrTls {
						newTls[k] = v
					}
				}
				Addrs[index]["tls"] = newTls
			}
		}
	}

	var links []string
	switch i.Type {
	case "socks":
		if uc, ok := userConfig["socks"]; ok {
			links = socksLink(uc, *inboundMap, Addrs)
		} else {
			log.Printf("LinkGenerator: socks user config not found for inbound %s", i.Tag)
		}
	case "http":
		if uc, ok := userConfig["http"]; ok {
			links = httpLink(uc, *inboundMap, Addrs)
		} else {
			log.Printf("LinkGenerator: http user config not found for inbound %s", i.Tag)
		}
	case "mixed":
		if ucSocks, ok := userConfig["socks"]; ok {
			links = append(links, socksLink(ucSocks, *inboundMap, Addrs)...)
		} else {
			log.Printf("LinkGenerator: mixed socks user config not found for inbound %s", i.Tag)
		}
		if ucHttp, ok := userConfig["http"]; ok {
			links = append(links, httpLink(ucHttp, *inboundMap, Addrs)...)
		} else {
			log.Printf("LinkGenerator: mixed http user config not found for inbound %s", i.Tag)
		}
	case "shadowsocks":
		links = shadowsocksLink(userConfig, *inboundMap, Addrs)
	case "naive":
		if uc, ok := userConfig["naive"]; ok {
			links = naiveLink(uc, *inboundMap, Addrs)
		} else {
			log.Printf("LinkGenerator: naive user config not found for inbound %s", i.Tag)
		}
	case "hysteria":
		if uc, ok := userConfig["hysteria"]; ok {
			links = hysteriaLink(uc, *inboundMap, Addrs)
		} else {
			log.Printf("LinkGenerator: hysteria user config not found for inbound %s", i.Tag)
		}
	case "hysteria2":
		if uc, ok := userConfig["hysteria2"]; ok {
			links = hysteria2Link(uc, *inboundMap, Addrs)
		} else {
			log.Printf("LinkGenerator: hysteria2 user config not found for inbound %s", i.Tag)
		}
	case "tuic":
		if uc, ok := userConfig["tuic"]; ok {
			links = tuicLink(uc, *inboundMap, Addrs)
		} else {
			log.Printf("LinkGenerator: tuic user config not found for inbound %s", i.Tag)
		}
	case "vless":
		if uc, ok := userConfig["vless"]; ok {
			links = vlessLink(uc, *inboundMap, Addrs)
		} else {
			log.Printf("LinkGenerator: vless user config not found for inbound %s", i.Tag)
		}
	case "anytls":
		if uc, ok := userConfig["anytls"]; ok {
			links = anytlsLink(uc, Addrs)
		} else {
			log.Printf("LinkGenerator: anytls user config not found for inbound %s", i.Tag)
		}
	case "trojan":
		if uc, ok := userConfig["trojan"]; ok {
			links = trojanLink(uc, *inboundMap, Addrs)
		} else {
			log.Printf("LinkGenerator: trojan user config not found for inbound %s", i.Tag)
		}
	case "vmess":
		if uc, ok := userConfig["vmess"]; ok {
			links = vmessLink(uc, *inboundMap, Addrs)
		} else {
			log.Printf("LinkGenerator: vmess user config not found for inbound %s", i.Tag)
		}
	default:
		log.Printf("LinkGenerator: unsupported inbound type %s for link generation for inbound %s", i.Type, i.Tag)
	}

	return links
}

func prepareTls(t *model.Tls) map[string]interface{} {
	var iTls, oTls map[string]interface{}
	if err := json.Unmarshal(t.Client, &oTls); err != nil {
		log.Printf("prepareTls: failed to unmarshal client TLS config: %v", err)
		return nil
	}
	if err := json.Unmarshal(t.Server, &iTls); err != nil {
		log.Printf("prepareTls: failed to unmarshal server TLS config: %v", err)
		return nil
	}

	for k, v := range iTls {
		switch k {
		case "enabled", "server_name", "alpn":
			oTls[k] = v
		case "reality":
			if reality, ok := v.(map[string]interface{}); ok {
				if clientReality, ok := oTls["reality"].(map[string]interface{}); ok {
					clientReality["enabled"] = reality["enabled"]
					if short_ids, hasSIds := reality["short_id"].([]interface{}); hasSIds && len(short_ids) > 0 {
						clientReality["short_id"] = short_ids[common.RandomInt(len(short_ids))]
					}
					oTls["reality"] = clientReality
				} else {
					log.Printf("prepareTls: 'reality' not found or not a map in client TLS config")
				}
			} else {
				log.Printf("prepareTls: 'reality' not found or not a map in server TLS config")
			}
		}
	}
	return oTls
}

func socksLink(userConfig map[string]interface{}, inbound map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	username, ok := userConfig["username"].(string)
	if !ok {
		log.Printf("socksLink: username not found or not a string in user config")
		return []string{}
	}
	password, ok := userConfig["password"].(string)
	if !ok {
		log.Printf("socksLink: password not found or not a string in user config")
		return []string{}
	}

	for _, addr := range addrs {
		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("socksLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("socksLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		links = append(links, fmt.Sprintf("socks5://%s:%s@%s:%d", username, password, server, uint(portFloat)))
	}
	return links
}

func httpLink(userConfig map[string]interface{}, inbound map[string]interface{}, addrs []map[string]interface{}) []string {
	var links []string
	username, ok := userConfig["username"].(string)
	if !ok {
		log.Printf("httpLink: username not found or not a string in user config")
		return []string{}
	}
	password, ok := userConfig["password"].(string)
	if !ok {
		log.Printf("httpLink: password not found or not a string in user config")
		return []string{}
	}

	for _, addr := range addrs {
		protocol := "http"
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls != nil {
			protocol = "https"
		}
		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("httpLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("httpLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		links = append(links, fmt.Sprintf("%s://%s:%s@%s:%d", protocol, username, password, server, uint(portFloat)))
	}
	return links
}

func shadowsocksLink(
	userConfig map[string]map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	var userPass []string
	method, ok := inbound["method"].(string)
	if !ok {
		log.Printf("shadowsocksLink: method not found or not a string in inbound config")
		return []string{}
	}

	if strings.HasPrefix(method, "2022") {
		inbPass, ok := inbound["password"].(string)
		if !ok {
			log.Printf("shadowsocksLink: inbound password not found or not a string for 2022 method")
			return []string{}
		}
		userPass = append(userPass, inbPass)
	}
	var pass string
	if method == "2022-blake3-aes-128-gcm" {
		if uc, ok := userConfig["shadowsocks16"]; ok {
			pass, ok = uc["password"].(string)
			if !ok {
				log.Printf("shadowsocksLink: shadowsocks16 password not found or not a string in user config")
				return []string{}
			}
		} else {
			log.Printf("shadowsocksLink: shadowsocks16 user config not found")
			return []string{}
		}
	} else {
		if uc, ok := userConfig["shadowsocks"]; ok {
			pass, ok = uc["password"].(string)
			if !ok {
				log.Printf("shadowsocksLink: shadowsocks password not found or not a string in user config")
				return []string{}
			}
		} else {
			log.Printf("shadowsocksLink: shadowsocks user config not found")
			return []string{}
		}
	}
	userPass = append(userPass, pass)

	uriBase := fmt.Sprintf("ss://%s", toBase64([]byte(fmt.Sprintf("%s:%s", method, strings.Join(userPass, ":")))))

	var links []string
	for _, addr := range addrs {
		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("shadowsocksLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("shadowsocksLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		remark, _ := addr["remark"].(string) // _ is ok here
		links = append(links, fmt.Sprintf("%s@%s:%.0f#%s", uriBase, server, portFloat, remark))
	}
	return links
}

func naiveLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, ok := userConfig["password"].(string)
	if !ok {
		log.Printf("naiveLink: password not found or not a string in user config")
		return []string{}
	}
	username, ok := userConfig["username"].(string)
	if !ok {
		log.Printf("naiveLink: username not found or not a string in user config")
		return []string{}
	}

	baseUri := "http2://"
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		params["padding"] = "1"
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls != nil {
			if sni, ok := tls["server_name"].(string); ok {
				params["peer"] = sni
			}
			if alpn, ok := tls["alpn"].([]interface{}); ok {
				alpnList := make([]string, len(alpn))
				for i, v := range alpn {
					if s, ok := v.(string); ok {
						alpnList[i] = s
					}
				}
				params["alpn"] = strings.Join(alpnList, ",")
			}
			if insecure, ok := tls["insecure"].(bool); ok && insecure {
				params["insecure"] = "1"
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params["tfo"] = "1"
		} else {
			params["tfo"] = "0"
		}

		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("naiveLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("naiveLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		remark, _ := addr["remark"].(string) // _ is ok here
		uri := baseUri + toBase64([]byte(fmt.Sprintf("%s:%s@%s:%.0f", username, password, server, portFloat)))
		links = append(links, addParams(uri, params, remark))
	}
	return links
}

func hysteriaLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	baseUri := "hysteria://"
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params["downmbps"] = fmt.Sprintf("%.0f", upmbps)
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params["upmbps"] = fmt.Sprintf("%.0f", downmbps)
		}
		if auth, ok := userConfig["auth_str"].(string); ok {
			params["auth"] = auth
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls != nil {
			getTlsParams(&params, tls, "insecure")
		}
		if obfs, ok := inbound["obfs"].(string); ok {
			params["obfs"] = obfs
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params["fastopen"] = "1"
		} else {
			params["fastopen"] = "0"
		}
		var outJson map[string]interface{}
		if outJsonRaw, ok := inbound["out_json"].(json.RawMessage); ok {
			if err := json.Unmarshal(outJsonRaw, &outJson); err != nil {
				log.Printf("hysteriaLink: failed to unmarshal out_json: %v", err)
			}
		}
		if mport, ok := outJson["server_ports"].([]interface{}); ok {
			mportList := make([]string, len(mport))
			for i, v := range mport {
				if s, ok := v.(string); ok {
					mportList[i] = s
				}
			}
			params["mport"] = strings.Join(mportList, ",")
		}

		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("hysteriaLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("hysteriaLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		remark, _ := addr["remark"].(string) // _ is ok here
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, server, portFloat)
		links = append(links, addParams(uri, params, remark))
	}

	return links
}

func hysteria2Link(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, ok := userConfig["password"].(string)
	if !ok {
		log.Printf("hysteria2Link: password not found or not a string in user config")
		return []string{}
	}
	baseUri := fmt.Sprintf("%s%s@", "hysteria2://", password)
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		if upmbps, ok := inbound["up_mbps"].(float64); ok {
			params["downmbps"] = fmt.Sprintf("%.0f", upmbps)
		}
		if downmbps, ok := inbound["down_mbps"].(float64); ok {
			params["upmbps"] = fmt.Sprintf("%.0f", downmbps)
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls != nil {
			getTlsParams(&params, tls, "insecure")
		}
		if obfs, ok := inbound["obfs"].(map[string]interface{}); ok && obfs != nil {
			if obfsType, ok := obfs["type"].(string); ok {
				params["obfs"] = obfsType
			}
			if obfsPassword, ok := obfs["password"].(string); ok {
				params["obfs-password"] = obfsPassword
			}
		}
		if tfo, ok := inbound["tcp_fast_open"].(bool); ok && tfo {
			params["fastopen"] = "1"
		} else {
			params["fastopen"] = "0"
		}
		var outJson map[string]interface{}
		if outJsonRaw, ok := inbound["out_json"].(json.RawMessage); ok {
			if err := json.Unmarshal(outJsonRaw, &outJson); err != nil {
				log.Printf("hysteria2Link: failed to unmarshal out_json: %v", err)
			}
		}
		if mport, ok := outJson["server_ports"].([]interface{}); ok {
			mportList := make([]string, len(mport))
			for i, v := range mport {
				if s, ok := v.(string); ok {
					mportList[i] = s
				}
			}
			params["mport"] = strings.Join(mportList, ",")
		}

		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("hysteria2Link: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("hysteria2Link: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		remark, _ := addr["remark"].(string) // _ is ok here
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, server, portFloat)
		links = append(links, addParams(uri, params, remark))
	}

	return links
}

func anytlsLink(
	userConfig map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, ok := userConfig["password"].(string)
	if !ok {
		log.Printf("anytlsLink: password not found or not a string in user config")
		return []string{}
	}
	baseUri := fmt.Sprintf("%s%s@", "anytls://", password)
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls != nil {
			getTlsParams(&params, tls, "insecure")
		}

		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("anytlsLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("anytlsLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		remark, _ := addr["remark"].(string) // _ is ok here
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, server, portFloat)
		links = append(links, addParams(uri, params, remark))
	}

	return links
}

func tuicLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	password, ok := userConfig["password"].(string)
	if !ok {
		log.Printf("tuicLink: password not found or not a string in user config")
		return []string{}
	}
	uuid, ok := userConfig["uuid"].(string)
	if !ok {
		log.Printf("tuicLink: uuid not found or not a string in user config")
		return []string{}
	}
	baseUri := fmt.Sprintf("%s%s:%s@", "tuic://", uuid, password)
	var links []string

	for _, addr := range addrs {
		params := map[string]string{}
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls != nil {
			getTlsParams(&params, tls, "insecure")
		}
		if congestionControl, ok := inbound["congestion_control"].(string); ok {
			params["congestion_control"] = congestionControl
		}

		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("tuicLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("tuicLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		remark, _ := addr["remark"].(string) // _ is ok here
		uri := fmt.Sprintf("%s%s:%.0f", baseUri, server, portFloat)
		links = append(links, addParams(uri, params, remark))
	}

	return links
}

func vlessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, ok := userConfig["uuid"].(string)
	if !ok {
		log.Printf("vlessLink: uuid not found or not a string in user config")
		return []string{}
	}
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := baseParams
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls != nil {
			if enabled, ok := tls["enabled"].(bool); ok && enabled {
				getTlsParams(&params, tls, "allowInsecure")
				if flow, ok := userConfig["flow"].(string); ok {
					params["flow"] = flow
				}
			}
		}
		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("vlessLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("vlessLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		remark, _ := addr["remark"].(string) // _ is ok here
		uri := fmt.Sprintf("vless://%s@%s:%.0f", uuid, server, portFloat)
		uri = addParams(uri, params, remark)
		links = append(links, uri)
	}

	return links
}

func trojanLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {
	password, ok := userConfig["password"].(string)
	if !ok {
		log.Printf("trojanLink: password not found or not a string in user config")
		return []string{}
	}
	baseParams := getTransportParams(inbound["transport"])
	var links []string

	for _, addr := range addrs {
		params := baseParams
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls != nil {
			if enabled, ok := tls["enabled"].(bool); ok && enabled {
				getTlsParams(&params, tls, "allowInsecure")
			}
		}
		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("trojanLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("trojanLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		remark, _ := addr["remark"].(string) // _ is ok here
		uri := fmt.Sprintf("trojan://%s@%s:%.0f", password, server, portFloat)
		uri = addParams(uri, params, remark)
		links = append(links, uri)
	}

	return links
}

func vmessLink(
	userConfig map[string]interface{},
	inbound map[string]interface{},
	addrs []map[string]interface{}) []string {

	uuid, ok := userConfig["uuid"].(string)
	if !ok {
		log.Printf("vmessLink: uuid not found or not a string in user config")
		return []string{}
	}
	trasportParams := getTransportParams(inbound["transport"])
	var links []string

	baseParams := map[string]interface{}{
		"v":   2,
		"id":  uuid,
		"aid": 0,
	}
	transportType := trasportParams["type"]

	if transportType == "http" || transportType == "tcp" {
		baseParams["net"] = "tcp"
		if transportType == "http" {
			baseParams["type"] = "http"
		}
	} else {
		baseParams["net"] = transportType
	}

	for _, addr := range addrs {
		obj := baseParams
		server, ok := addr["server"].(string)
		if !ok {
			log.Printf("vmessLink: server not found or not a string in addr: %+v", addr)
			continue
		}
		obj["add"] = server
		portFloat, ok := addr["server_port"].(float64)
		if !ok {
			log.Printf("vmessLink: server_port not found or not a number in addr: %+v", addr)
			continue
		}
		obj["port"] = uint(portFloat)
		remark, ok := addr["remark"].(string)
		if !ok {
			log.Printf("vmessLink: remark not found or not a string in addr: %+v", addr)
			remark = "" // Default to empty string if remark is missing
		}
		obj["ps"] = remark

		if host, ok := trasportParams["host"]; ok && host != "" {
			obj["host"] = host
		}
		if path, ok := trasportParams["path"]; ok && path != "" {
			obj["path"] = path
		}
		if tls, ok := addr["tls"].(map[string]interface{}); ok && tls != nil {
			if enabled, ok := tls["enabled"].(bool); ok && enabled {
				obj["tls"] = "tls"
				if insecure, ok := tls["insecure"].(bool); ok && insecure {
					obj["allowInsecure"] = 1
				}
				if sni, ok := tls["server_name"].(string); ok {
					obj["sni"] = sni
				}
				if alpn, ok := tls["alpn"].([]interface{}); ok {
					alpnList := make([]string, len(alpn))
					for i, v := range alpn {
						if s, ok := v.(string); ok {
							alpnList[i] = s
						}
					}
					obj["alpn"] = strings.Join(alpnList, ",")
				}
				if utls, ok := tls["utls"].(map[string]interface{}); ok {
					if fingerprint, ok := utls["fingerprint"].(string); ok {
						obj["fp"] = fingerprint
					}
				}
			}
		} else {
			obj["tls"] = "none"
		}

		jsonStr, err := json.Marshal(obj) // Use Marshal instead of MarshalIndent for link generation
		if err != nil {
			log.Printf("vmessLink: failed to marshal VMess config object: %v", err)
			continue
		}

		uri := fmt.Sprintf("vmess://%s", toBase64(jsonStr))
		links = append(links, uri)
	}
	return links
}

func toBase64(d []byte) string {
	return base64.StdEncoding.EncodeToString(d) // Removed redundant []byte(d)
}

func addParams(uri string, params map[string]string, remark string) string {
	URL, err := url.Parse(uri)
	if err != nil {
		log.Printf("addParams: failed to parse URI %s: %v", uri, err)
		return uri // Return original URI if parsing fails
	}
	q := URL.Query() // Use URL.Query() to get a mutable copy of query parameters
	for k, v := range params {
		switch k {
		case "mport", "alpn":
			q.Add(k, v)
		default:
			q.Add(k, v)
		}
	}
	URL.RawQuery = q.Encode() // Encode the query parameters back to RawQuery
	URL.Fragment = remark
	return URL.String()
}

func getTransportParams(t interface{}) map[string]string {
	params := map[string]string{}
	trasport, ok := t.(map[string]interface{})
	if !ok {
		log.Printf("getTransportParams: transport config is not a map: %+v", t)
		params["type"] = "tcp" // Default to tcp
		return params
	}

	transportType, ok := trasport["type"].(string)
	if !ok {
		log.Printf("getTransportParams: transport type not found or not a string in transport config: %+v", trasport)
		params["type"] = "tcp" // Default to tcp
		return params
	}
	params["type"] = transportType

	switch params["type"] {
	case "http":
		if host, ok := trasport["host"].([]interface{}); ok {
			var hosts []string
			for _, v := range host {
				if s, ok := v.(string); ok {
					hosts = append(hosts, s)
				}
			}
			params["host"] = strings.Join(hosts, ",")
		}
		if path, ok := trasport["path"].(string); ok {
			params["path"] = path
		}
	case "ws":
		if path, ok := trasport["path"].(string); ok {
			params["path"] = path
		}
		if headers, ok := trasport["headers"].(map[string]interface{}); ok {
			if host, ok := headers["Host"].(string); ok {
				params["host"] = host
			}
		}
	case "grpc":
		if serviceName, ok := trasport["service_name"].(string); ok {
			params["serviceName"] = serviceName
		}
	case "httpupgrade":
		if host, ok := trasport["host"].(string); ok {
			params["host"] = host
		}
		if path, ok := trasport["path"].(string); ok {
			params["path"] = path
		}
	}
	return params
}

func getTlsParams(params *map[string]string, tls map[string]interface{}, insecureKey string) {
	if reality, ok := tls["reality"].(map[string]interface{}); ok && reality != nil {
		if enabled, ok := reality["enabled"].(bool); ok && enabled {
			(*params)["security"] = "reality"
			if pbk, ok := reality["public_key"].(string); ok {
				(*params)["pbk"] = pbk
			}
			if sid, ok := reality["short_id"].(string); ok {
				(*params)["sid"] = sid
			}
		}
	} else {
		(*params)["security"] = "tls"
		if insecure, ok := tls["insecure"].(bool); ok && insecure {
			(*params)[insecureKey] = "1"
		}
		if disableSni, ok := tls["disable_sni"].(bool); ok && disableSni {
			(*params)["disable_sni"] = "1"
		}
	}
	if utls, ok := tls["utls"].(map[string]interface{}); ok && utls != nil {
		if fingerprint, ok := utls["fingerprint"].(string); ok {
			(*params)["fp"] = fingerprint
		}
	}
	if sni, ok := tls["server_name"].(string); ok {
		(*params)["sni"] = sni
	}
	if alpn, ok := tls["alpn"].([]interface{}); ok {
		alpnList := make([]string, len(alpn))
		for i, v := range alpn {
			if s, ok := v.(string); ok {
				alpnList[i] = s
			}
		}
		(*params)["alpn"] = strings.Join(alpnList, ",")
	}
}
