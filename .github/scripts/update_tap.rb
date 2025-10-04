#!/usr/bin/env ruby
# frozen_string_literal: true

require 'pathname'

FORMULA_TEMPLATE = <<~'FORMULA'
  class Rocketship < Formula
    desc "Rocketship CLI"
    homepage "https://github.com/rocketship-ai/rocketship"
    version "__VERSION__"

    on_macos do
      on_arm do
        url "__URL_DARWIN_ARM64__"
        sha256 "__SHA_DARWIN_ARM64__"
      end
      on_intel do
        url "__URL_DARWIN_AMD64__"
        sha256 "__SHA_DARWIN_AMD64__"
      end
    end

    on_linux do
      on_arm do
        url "__URL_LINUX_ARM64__"
        sha256 "__SHA_LINUX_ARM64__"
      end
      on_intel do
        url "__URL_LINUX_AMD64__"
        sha256 "__SHA_LINUX_AMD64__"
      end
    end

    def install
      target = if OS.mac?
                "rocketship-darwin-#{Hardware::CPU.arm? ? "arm64" : "amd64"}"
              else
                "rocketship-linux-#{Hardware::CPU.arm? ? "arm64" : "amd64"}"
              end
      bin.install target => "rocketship"
    end

    test do
      system "#{bin}/rocketship", "--version"
    end
  end
FORMULA

REQUIRED_ENV = {
  '__VERSION__' => 'TAG',
  '__URL_DARWIN_ARM64__' => 'URL_ROCKETSHIP_DARWIN_ARM64',
  '__SHA_DARWIN_ARM64__' => 'SHA_ROCKETSHIP_DARWIN_ARM64',
  '__URL_DARWIN_AMD64__' => 'URL_ROCKETSHIP_DARWIN_AMD64',
  '__SHA_DARWIN_AMD64__' => 'SHA_ROCKETSHIP_DARWIN_AMD64',
  '__URL_LINUX_ARM64__' => 'URL_ROCKETSHIP_LINUX_ARM64',
  '__SHA_LINUX_ARM64__' => 'SHA_ROCKETSHIP_LINUX_ARM64',
  '__URL_LINUX_AMD64__' => 'URL_ROCKETSHIP_LINUX_AMD64',
  '__SHA_LINUX_AMD64__' => 'SHA_ROCKETSHIP_LINUX_AMD64',
}.freeze

formula = FORMULA_TEMPLATE.dup

REQUIRED_ENV.each do |placeholder, env_key|
  value = ENV[env_key]
  abort "Missing environment variable: #{env_key}" if value.nil? || value.empty?
  formula.gsub!(placeholder, value)
end

Pathname.new('Formula').mkpath
Pathname.new('Formula/rocketship.rb').write(formula)
