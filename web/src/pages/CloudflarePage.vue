<script setup lang="ts">
import CloudflarePanel from '../components/CloudflarePanel.vue'
import { useDashboardContext } from '../dashboardContext'

const { connections, reload } = useDashboardContext()
</script>

<template>
  <div class="page-stack cloudflare-page">
    <header class="page-heading"><div><p class="eyebrow">STEP 01 · CLOUDFLARE</p><h1>先连接 Cloudflare</h1><p>按最小权限创建 Token，验证可见 Zone，再进入映射服务。</p></div></header>
    <CloudflarePanel :connections="connections" @changed="reload" />
  </div>
</template>
