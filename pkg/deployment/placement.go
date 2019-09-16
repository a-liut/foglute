package deployment

import (
	"fmt"
	"foglute/internal/model"
	"math/rand"
)

// Returns the best placement from a list of placements
func pickBestPlacement(placements []model.Placement) (*model.Placement, error) {
	if len(placements) == 0 {
		return nil, fmt.Errorf("no feasible deployments")
	}

	// Scan placements and pick the best ones
	list := make([]*model.Placement, 0)
	bestProb := -1.0
	for _, p := range placements {
		if bestProb < p.Probability {
			// Clear the list
			list = list[:0]

			list = append(list, &p)

			bestProb = p.Probability
		} else if bestProb == p.Probability {
			list = append(list, &p)
		}
	}

	// Pick a random placement
	idx := rand.Intn(len(list))

	return list[idx], nil
}
