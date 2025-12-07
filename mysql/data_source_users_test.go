package mysql

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataSourceUsers(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUsersConfigBasic("%", "%"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.mysql_users.test", "user_pattern", "%"),
					resource.TestCheckResourceAttr("data.mysql_users.test", "host_pattern", "%"),
					testAccUsersCount("data.mysql_users.test", "users.#", func(rn string, userCount int) error {
						if userCount < 1 {
							return fmt.Errorf("%s: users not found", rn)
						}

						return nil
					}),
				),
			},
			{
				Config: testAccUsersConfigBasic("__user_does_not_exist__", "__host_does_not_exist__"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.mysql_users.test", "user_pattern", "__user_does_not_exist__"),
					resource.TestCheckResourceAttr("data.mysql_users.test", "host_pattern", "__host_does_not_exist__"),
					testAccUsersCount("data.mysql_users.test", "users.#", func(rn string, userCount int) error {
						if userCount > 0 {
							return fmt.Errorf("%s: unexpected user found", rn)
						}

						return nil
					}),
				),
			},
			{
				Config: testAccUsersConfigUserPattern("%"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.mysql_users.test", "user_pattern", "%"),
					testAccUsersCount("data.mysql_users.test", "users.#", func(rn string, userCount int) error {
						if userCount < 1 {
							return fmt.Errorf("%s: users not found", rn)
						}

						return nil
					}),
				),
			},
			{
				Config: testAccUsersConfigHostPattern("%"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.mysql_users.test", "host_pattern", "%"),
					testAccUsersCount("data.mysql_users.test", "users.#", func(rn string, userCount int) error {
						if userCount < 1 {
							return fmt.Errorf("%s: users not found", rn)
						}

						return nil
					}),
				),
			},
			{
				Config: testAccUsersConfigExclude([]string{"root@localhost"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.mysql_users.test", "exclude_users.#", "1"),
					testAccUsersCount("data.mysql_users.test", "users.#", func(rn string, userCount int) error {
						if userCount < 1 {
							return fmt.Errorf("%s: users not found", rn)
						}
						return nil
					}),
				),
			},
		},
	})
}

func testAccUsersCount(rn string, key string, check func(string, int) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]

		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		value, ok := rs.Primary.Attributes[key]

		if !ok {
			return fmt.Errorf("%s: attribute '%s' not found", rn, key)
		}

		usersCount, err := strconv.Atoi(value)

		if err != nil {
			return err
		}

		return check(rn, usersCount)
	}
}

func testAccUsersConfigBasic(userPattern, hostPattern string) string {
	return fmt.Sprintf(`
data "mysql_users" "test"{
	user_pattern = "%s"
	host_pattern = "%s"
}`, userPattern, hostPattern)
}

func testAccUsersConfigUserPattern(userPattern string) string {
	return fmt.Sprintf(`
data "mysql_users" "test"{
	user_pattern = "%s"
}`, userPattern)
}

func testAccUsersConfigHostPattern(hostPattern string) string {
	return fmt.Sprintf(`
data "mysql_users" "test"{
	host_pattern = "%s"
}`, hostPattern)
}

func testAccUsersConfigExclude(excludeUsers []string) string {
	var excludeUsersStr []string
	for _, user := range excludeUsers {
		excludeUsersStr = append(excludeUsersStr, fmt.Sprintf("%q", user))
	}
	return fmt.Sprintf(`
data "mysql_users" "test"{
	exclude_users = [%s]
}`, strings.Join(excludeUsersStr, ","))
}
