#!/usr/bin/env ruby
# frozen_string_literal: true

require "yaml"

path = ARGV.fetch(0) do
  warn "usage: #{File.basename($PROGRAM_NAME)} <manifest.yaml>"
  exit 2
end

begin
  documents = YAML.load_stream(File.read(path)).compact
rescue StandardError => e
  warn "invalid Kubernetes YAML #{path}: #{e.message}"
  exit 1
end

if documents.empty?
  warn "invalid Kubernetes YAML #{path}: no documents found"
  exit 1
end

documents.each_with_index do |document, index|
  unless document.is_a?(Hash)
    warn "invalid Kubernetes YAML #{path}: document #{index + 1} is not a mapping"
    exit 1
  end

  api_version = document["apiVersion"]
  kind = document["kind"]
  metadata = document["metadata"]
  name = metadata.is_a?(Hash) ? metadata["name"] : nil

  if api_version.to_s.empty? || kind.to_s.empty? || name.to_s.empty?
    warn "invalid Kubernetes YAML #{path}: document #{index + 1} must define apiVersion, kind, and metadata.name"
    exit 1
  end
end

puts "validated #{documents.length} Kubernetes document(s) from #{path}"
