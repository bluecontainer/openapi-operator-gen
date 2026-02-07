# Makefile for rundeck-k8s-nodes Rundeck plugin
PLUGIN_NAME := rundeck-k8s-nodes
VERSION := 1.0.0
PLUGIN_ZIP := $(PLUGIN_NAME)-$(VERSION).zip

.PHONY: all clean build

all: build

build: $(PLUGIN_ZIP)

$(PLUGIN_ZIP): plugin.yaml contents/nodes.sh
	@echo "Building Rundeck plugin: $(PLUGIN_ZIP)"
	chmod +x contents/nodes.sh
	zip -r $(PLUGIN_ZIP) plugin.yaml contents/

clean:
	rm -f $(PLUGIN_ZIP)

# Install to Rundeck libext directory (requires RUNDECK_HOME)
install: $(PLUGIN_ZIP)
ifndef RUNDECK_HOME
	$(error RUNDECK_HOME is not set)
endif
	cp $(PLUGIN_ZIP) $(RUNDECK_HOME)/libext/
	@echo "Plugin installed to $(RUNDECK_HOME)/libext/"
