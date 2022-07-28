require "pathname"

installdir = Pathname.new(__FILE__).join("..")
ENV['GOBIN'] = installdir.join("bin").to_s

desc "Build helper program"
task :helper do
	sh "go install github.com/speedata/boxesandglue/helper"
end

desc "Create pattern map"
task :genpatterns => :helper do
	sh "bin/helper"
end
