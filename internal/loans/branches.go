package loans

import "github.com/gin-gonic/gin"
import "net/http"

// Branch is a physical location a customer can walk into — needed for Gold
// Loan, where the collateral itself (not just paperwork) has to be
// physically appraised and held in a vault, unlike every other loan
// category here which is handled entirely remotely. This is a fixed list,
// not a database table — there is no branch-management admin screen in
// this system, and standing one up is out of scope for what Gold Loan's
// apply form actually needs: somewhere for the customer to say where
// they'll bring the gold.
type Branch struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	City string `json:"city"`
}

var branches = []Branch{
	{ID: "mumbai-andheri", Name: "Andheri Branch", City: "Mumbai"},
	{ID: "mumbai-borivali", Name: "Borivali Branch", City: "Mumbai"},
	{ID: "pune-shivajinagar", Name: "Shivajinagar Branch", City: "Pune"},
	{ID: "delhi-cp", Name: "Connaught Place Branch", City: "Delhi"},
	{ID: "bangalore-indiranagar", Name: "Indiranagar Branch", City: "Bangalore"},
}

// ListBranchesHandler handles GET /branches — used by the Gold Loan apply
// form to let the customer pick where they'll bring their gold in.
func ListBranchesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, branches)
}
