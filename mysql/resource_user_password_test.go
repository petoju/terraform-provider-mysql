package mysql

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"testing"
)

func TestAccUserPassword_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserPasswordConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user_password.test", "user", "jdoe"),
					resource.TestCheckResourceAttrSet("mysql_user_password.test", "plaintext_password"),
				),
			},
		},
	})
}

const testAccUserPasswordConfig_basic = `
resource "mysql_user" "test" {
  user = "jdoe"
}

resource "mysql_user_password" "test" {
  user               = "${mysql_user.test.user}"
  plaintext_password = "somepass"
}
`
