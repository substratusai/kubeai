package endpoints

func (g *group) getAddrLeastLoad(adapter string) (endpoint, bool) {
	var bestEp endpoint
	var found bool
	var minInFlight int
	for _, ep := range g.endpoints {
		if adapter != "" {
			// Skip endpoints that don't have the requested adapter.
			if _, ok := ep.adapters[adapter]; !ok {
				continue
			}
		}
		inFlight := int(ep.inFlight.Load())
		if !found || inFlight < minInFlight {
			bestEp = ep
			found = true
			minInFlight = inFlight
		}
	}

	return bestEp, found
}
