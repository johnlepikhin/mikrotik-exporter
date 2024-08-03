package collector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-routeros/routeros/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type lteCollector struct {
	props        []string
	descriptions map[string]*prometheus.Desc
}

func newLteCollector() routerOSCollector {
	c := &lteCollector{}
	c.init()
	return c
}

func (c *lteCollector) init() {
	c.props = []string{"current-cellid", "primary-band", "ca-band", "rssi", "rsrp", "rsrq", "sinr", "lac", "sector-id", "phy-cellid", "cqi", "session-uptime"}
	labelNames := []string{"name", "address", "interface", "cellid", "primaryband", "caband"}
	c.descriptions = make(map[string]*prometheus.Desc)
	for _, p := range c.props {
		c.descriptions[p] = descriptionForPropertyName("lte_interface", p, labelNames)
	}
}

func (c *lteCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}
}

func (c *lteCollector) collect(ctx *collectorContext) error {
	names, err := c.fetchInterfaceNames(ctx)
	if err != nil {
		return err
	}

	for _, n := range names {
		err := c.collectForInterface(n, ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *lteCollector) fetchInterfaceNames(ctx *collectorContext) ([]string, error) {
	log.WithFields(log.Fields{
		"device": ctx.device.Name,
	}).Info("fetching interface names")
	reply, err := ctx.client.Run("/interface/lte/print", "=.proplist=name")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching lte interface names")
		return nil, err
	}

	names := []string{}
	for _, re := range reply.Re {
		names = append(names, re.Map["name"])
	}

	log.WithFields(log.Fields{
		"device": ctx.device.Name,
		"names":  names,
	}).Infof("fetched lte interface names: %v", names)

	return names, nil
}

func (c *lteCollector) collectForInterface(iface string, ctx *collectorContext) error {
	log.WithFields(log.Fields{
		"interface": iface,
		"device":    ctx.device.Name,
		"proplist":  strings.Join(c.props, ","),
	}).Info("fetching stats")
	reply, err := ctx.client.Run("/interface/lte/monitor", "=once=", fmt.Sprintf("=.id=%s", iface), "=.proplist="+strings.Join(c.props, ","))
	//	reply, err := ctx.client.Run("/interface/lte/monitor", fmt.Sprintf("=numbers=lte1"), "=once=", "=.proplist="+strings.Join(c.props, ","))
	//	reply, err := ctx.client.Run("/interface/lte/monitor", fmt.Sprintf("=number=%s", iface), "=once=")
	if err != nil {
		log.WithFields(log.Fields{
			"interface": iface,
			"device":    ctx.device.Name,
			"error":     err,
		}).Error("error fetching interface statistics - LTE")
		return err
	}

	log.WithFields(log.Fields{
		"interface": iface,
		"device":    ctx.device.Name,
		"error":     err,
	}).Infof("Got reply: %v", reply)

	for _, p := range c.props[3:] {
		// panic: runtime error: index out of range [0] with length 0
		if reply.Re == nil || len(reply.Re) == 0 {
			continue
		}
		// there's always going to be only one sentence in reply, as we
		// have to explicitly specify the interface
		c.collectMetricForProperty(p, iface, reply.Re[0], ctx)
	}

	return nil
}

func (c *lteCollector) collectMetricForProperty(property, iface string, re *proto.Sentence, ctx *collectorContext) {
	desc := c.descriptions[property]
	current_cellid := re.Map["current-cellid"]
	// get only band and its width, drop earfcn and phy-cellid info
	primaryband := re.Map["primary-band"]
	if primaryband != "" {
		primaryband = strings.Fields(primaryband)[0]
	}
	caband := re.Map["ca-band"]
	if caband != "" {
		caband = strings.Fields(caband)[0]
	}

	if re.Map[property] == "" {
		return
	}

	if property == "session-uptime" {
		v, err := parseDuration(re.Map[property])
		if err != nil {
			log.WithFields(log.Fields{
				"property":  property,
				"interface": iface,
				"device":    ctx.device.Name,
				"error":     err,
			}).Error("error parsing interface duration metric value")
			return
		}
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, v, ctx.device.Name, ctx.device.Address, iface, current_cellid, primaryband, caband)
		return
	} else {
		v, err := strconv.ParseFloat(re.Map[property], 64)
		if err != nil {
			log.WithFields(log.Fields{
				"property":  property,
				"interface": iface,
				"device":    ctx.device.Name,
				"error":     err,
			}).Error("error parsing interface metric value")
			return
		}
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, ctx.device.Name, ctx.device.Address, iface, current_cellid, primaryband, caband)
	}
}
