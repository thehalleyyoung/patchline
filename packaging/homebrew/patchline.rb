class Patchline < Formula
  desc "Deterministic checker for data-change repair evidence"
  homepage "https://github.com/thehalleyyoung/patchline"
  version "0.0.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/thehalleyyoung/patchline/releases/download/v0.0.0/patchline_Darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_DARWIN_ARM64_SHA256"
    else
      url "https://github.com/thehalleyyoung/patchline/releases/download/v0.0.0/patchline_Darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_DARWIN_AMD64_SHA256"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/thehalleyyoung/patchline/releases/download/v0.0.0/patchline_Linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_LINUX_ARM64_SHA256"
    else
      url "https://github.com/thehalleyyoung/patchline/releases/download/v0.0.0/patchline_Linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_LINUX_AMD64_SHA256"
    end
  end

  def install
    bin.install "patchline"
  end

  test do
    system "#{bin}/patchline", "--help"
  end
end
