class Upctl < Formula
  desc "CLI tool for setting up local development environments using Kubernetes or Docker Compose"
  homepage "https://github.com/uptime-labs/upctl"
  license "MIT"  # Update this based on your actual license

  # Update with your latest version
  version "0.8.0"  # Update based on your latest release version

  if OS.mac?
    if Hardware::CPU.arm?
      url "https://github.com/uptime-labs/upctl/releases/download/v#{version}/upctl_#{version}_darwin_arm64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"  # Replace with actual SHA256 from your build
    else
      url "https://github.com/uptime-labs/upctl/releases/download/v#{version}/upctl_#{version}_darwin_amd64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"  # Replace with actual SHA256 from your build
    end
  elsif OS.linux?
    if Hardware::CPU.arm?
      url "https://github.com/uptime-labs/upctl/releases/download/v#{version}/upctl_#{version}_linux_arm64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"  # Replace with actual SHA256 from your build
    else
      url "https://github.com/uptime-labs/upctl/releases/download/v#{version}/upctl_#{version}_linux_amd64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256"  # Replace with actual SHA256 from your build
    end
  end

  depends_on "kubectl" => :recommended
  depends_on "mysql-client" => :recommended
  depends_on "awscli" => :recommended
  depends_on "docker" => :recommended
  depends_on "helm" => :recommended

  def install
    bin.install Dir["*"].first => "upctl"
  end

  def post_install
    (buildpath/"upctl.yaml").write <<~EOS
      repositories:
        - name: example
          url: https://example.com/charts
      
      packages:
        - name: example
          chart: example/example
          version: 1.0.0
      
      docker_compose:
        version: '3.8'
        services:
          example:
            image: example/example:latest
            ports:
              - "8080:8080"
            restart: unless-stopped
    EOS

    mkdir_p "#{Dir.home}/.upctl"
    cp "upctl.yaml", "#{Dir.home}/.upctl.yaml" unless File.exist?("#{Dir.home}/.upctl.yaml")
  end

  test do
    assert_match "upctl version", shell_output("#{bin}/upctl version")
  end
end