package network

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestIsIpInSubnet(t *testing.T) {
	ctx := context.Background()
	ip1 := "192.168.0.5"
	ip2 := "125.216.250.89"
	subnet := "192.168.0.0/24"
	Convey("TestIsIpInSubnet", t, func() {
		So(isIpInSubnet(ctx, ip1, subnet), ShouldBeTrue)
		So(isIpInSubnet(ctx, ip2, subnet), ShouldBeFalse)
	})
}

func TestIsValidSubnets(t *testing.T) {
	Convey("TestIsValidSubnets", t, func() {
		So(IsValidSubnets("192.168.0.0/24, 10.0.0.0/8"), ShouldBeNil)
		So(IsValidSubnets("192.168.0.0/24, invalid-subnet"), ShouldNotBeNil)
	})
}

func TestIsIpInSubnets(t *testing.T) {
	ctx := context.Background()

	Convey("TestIsIpInSubnets", t, func() {
		subnets := "192.168.0.0/24, 10.0.0.0/8"

		So(IsIpInSubnets(ctx, "10.1.2.3", subnets), ShouldBeTrue)
		So(IsIpInSubnets(ctx, "172.16.0.1", subnets), ShouldBeFalse)
	})
}
