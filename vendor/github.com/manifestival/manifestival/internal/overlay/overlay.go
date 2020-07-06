package overlay

// Overlays the values of src onto tgt
func Copy(src, tgt map[string]interface{}) {
	for k, v := range src {
		switch y := tgt[k].(type) {
		case map[string]interface{}:
			if x, ok := v.(map[string]interface{}); ok {
				Copy(x, y)
			} else {
				tgt[k] = v
			}
		case []interface{}:
			if x, ok := v.([]interface{}); ok {
				if len(y) < len(x) {
					tgt[k] = v
					continue
				}
				for i := range x {
					xi, xok := x[i].(map[string]interface{})
					yi, yok := y[i].(map[string]interface{})
					if xok && yok {
						Copy(xi, yi)
					} else {
						y[i] = x[i]
					}
				}
				tgt[k] = y[:len(x)]
			} else {
				tgt[k] = v
			}
		default:
			tgt[k] = v
		}
	}
}
