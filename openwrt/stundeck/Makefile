include $(TOPDIR)/rules.mk

PKG_NAME:=stundeck
PKG_VERSION:=$(if $(STUNDECK_VERSION),$(STUNDECK_VERSION),0.1.0)
PKG_RELEASE:=1

PKG_MAINTAINER:=dabird
PKG_LICENSE:=Apache-2.0

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)

# 二进制已经带 -s -w，别让 sstrip 再动一次
RSTRIP:=:
STRIP:=:

include $(INCLUDE_DIR)/package.mk

define Package/stundeck
  SECTION:=net
  CATEGORY:=Network
  TITLE:=StunDeck - STUN/NATMap + Cloudflare redirect controller
  URL:=https://github.com/Nciae-Zyh/stundeck
  DEPENDS:=@aarch64 +natmap +ca-bundle
endef

define Package/stundeck/conffiles
/etc/config/stundeck
endef

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
endef

Build/Configure:=
Build/Compile:=

define Package/stundeck/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) $(STUNDECK_BIN_DIR)/stundeck $(1)/usr/bin/stundeck
	$(INSTALL_BIN) $(STUNDECK_BIN_DIR)/stundeck-notify $(1)/usr/bin/stundeck-notify
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/stundeck.init $(1)/etc/init.d/stundeck
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/stundeck.config $(1)/etc/config/stundeck
endef

$(eval $(call BuildPackage,stundeck))
