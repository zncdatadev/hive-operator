package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	hivev1alpha1 "github.com/zncdatadev/hive-operator/api/v1alpha1"
)

// Generates log4j2 properties template correctly with valid LoggingConfigSpec input
var _ = Describe("MetastoreLoggingRecociler", func() {
	Context("when loggingConfig is provided", func() {
		It("should generate log4j2 properties template correctly", func() {
			// Arrange
			loggingConfig := &hivev1alpha1.ContainerLoggingSpec{
				Metastore: &hivev1alpha1.LoggingConfigSpec{
					Console: &hivev1alpha1.LogLevelSpec{Level: "INFO"},
					File:    &hivev1alpha1.LogLevelSpec{Level: "DEBUG"},
					Loggers: map[string]*hivev1alpha1.LogLevelSpec{
						"com.example": {Level: "WARN"},
						"com.abc":     {Level: "ERROR"},
					},
				},
			}

			reconciler := &MetastoreLoggingRecociler{}

			// Act
			result := reconciler.metastoreLog4j(loggingConfig)

			// Assert
			Expect(result).To(ContainSubstring("logger.com.example.level = WARN"))
			Expect(result).To(ContainSubstring("logger.com.abc.level = ERROR"))
			Expect(result).To(ContainSubstring("loggers = ")) // hash map order attenction, so not complete yet

			Expect(result).To(ContainSubstring("appender.FILE.filter.threshold.level = DEBUG"))
			Expect(result).To(ContainSubstring("appender.CONSOLE.filter.threshold.level = INFO"))

			Expect(result).To(ContainSubstring("rootLogger.level = INFO"))
			Expect(result).To(ContainSubstring("rootLogger.appenderRefs = CONSOLE, FILE"))
			Expect(result).To(ContainSubstring("rootLogger.appenderRef.CONSOLE.ref = CONSOLE"))
			Expect(result).To(ContainSubstring("rootLogger.appenderRef.FILE.ref = FILE"))
		})
	})

	Context("when loggingConfig.Loggers is empty", func() {
		It("should handle empty loggers gracefully", func() {
			// Arrange
			loggingConfig := &hivev1alpha1.ContainerLoggingSpec{}
			r := &MetastoreLoggingRecociler{}

			// Act
			result := r.metastoreLog4j(loggingConfig)

			// Assert
			Expect(result).To(ContainSubstring("appender.FILE.filter.threshold.level = INFO"))
			Expect(result).To(ContainSubstring("appender.CONSOLE.filter.threshold.level = INFO"))
			Expect(result).NotTo(ContainSubstring("loggers="))
		})
	})

	Context("when loggingConfig fields are missing", func() {
		It("should handle missing fields in loggingConfig without errors", func() {
			// Arrange
			// loggingConfig := &hivev1alpha1.LoggingConfigSpec{
			// 	Console: &hivev1alpha1.LogLevelSpec{},
			// 	File:    &hivev1alpha1.LogLevelSpec{},
			// 	Loggers: map[string]*hivev1alpha1.LogLevelSpec{},
			// }

			loggingConfig := &hivev1alpha1.ContainerLoggingSpec{
				Metastore: &hivev1alpha1.LoggingConfigSpec{
					Loggers: map[string]*hivev1alpha1.LogLevelSpec{},
					Console: &hivev1alpha1.LogLevelSpec{},
					File:    &hivev1alpha1.LogLevelSpec{},
				},
			}

			r := &MetastoreLoggingRecociler{}

			// Act
			result := r.metastoreLog4j(loggingConfig)

			// Assert
			Expect(result).To(ContainSubstring("appender.FILE.filter.threshold.level = INFO"))
			Expect(result).To(ContainSubstring("appender.CONSOLE.filter.threshold.level = INFO"))
			Expect(result).NotTo(ContainSubstring("loggers="))
		})
	})
})

var _ = Describe("GetLoggerLevel", func() {
	Context("when condition is true and trueValFunc returns a non-empty string", func() {
		It("should return trueVal", func() {
			trueValFunc := func() string { return "non-empty" }
			defaultVal := "default"

			result := GetLoggerLevel(true, trueValFunc, defaultVal)

			Expect(result).To(Equal("non-empty"))
		})
	})

	Context("when condition is false", func() {
		It("should return defaultVal", func() {
			trueValFunc := func() string { return "non-empty" }
			defaultVal := "default"

			result := GetLoggerLevel(false, trueValFunc, defaultVal)

			Expect(result).To(Equal("default"))
		})
	})

	Context("when condition is true but trueValFunc returns an empty string", func() {
		It("should return defaultVal", func() {
			trueValFunc := func() string { return "" }
			defaultVal := "default"

			result := GetLoggerLevel(true, trueValFunc, defaultVal)

			Expect(result).To(Equal("default"))
		})
	})

	Context("when trueValFunc returns a string with only whitespace", func() {
		It("should return defaultVal", func() {
			trueValFunc := func() string { return "   " }
			defaultVal := "default"

			result := GetLoggerLevel(true, trueValFunc, defaultVal)

			Expect(result).To(Equal("default"))
		})
	})

	Context("when trueValFunc returns a string with special characters", func() {
		It("should return the string with special characters from trueValFunc", func() {
			trueValFunc := func() string { return "!@#$%^&*()" }
			defaultVal := "default"

			result := GetLoggerLevel(true, trueValFunc, defaultVal)

			Expect(result).To(Equal("!@#$%^&*()"))
		})
	})
})
