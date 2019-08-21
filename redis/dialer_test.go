package redis_test

import (
	"github.com/jamillosantos/go-graphqlws/redis"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dialer", func() {
	It("should create dialer", func() {
		dialer := redis.NewDialer("tcp", "localhost:6379")
		Expect(dialer).ToNot(BeNil())
	})

	It("should create a new connection", func() {
		dialer := redis.NewDialer("tcp", "localhost:6379")

		conn, err := dialer.Dial()
		Expect(err).ToNot(HaveOccurred())
		_, err = conn.Do("PING")
		Expect(err).ToNot(HaveOccurred())
		Expect(conn.Close()).To(Succeed())
	})

	It("should create multiple connections", func() {
		dialer := redis.NewDialer("tcp", "localhost:6379")

		for i := 0; i < 1000; i++ {
			conn, err := dialer.Dial()
			Expect(err).ToNot(HaveOccurred())
			_, err = conn.Do("PING")
			Expect(err).ToNot(HaveOccurred())
			Expect(conn.Close()).To(Succeed())
		}
	})
})
