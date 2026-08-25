<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'
import type { CloudflareConnection, Zone } from '../types'

const props = defineProps<{ connections: CloudflareConnection[] }>()
const emit = defineEmits<{ changed: [] }>()

const name = ref('Cloudflare')
const token = ref('')
const zones = ref<Zone[]>([])
const zoneId = ref('')
const busy = ref(false)
const message = ref('')
const error = ref('')
const selectedZone = computed(() => zones.value.find((zone) => zone.id === zoneId.value))
const permissionText = 'Zone > Zone > Read\nZone > DNS > Edit\nZone > Single Redirect > Edit\nZone Resources > Include > Specific zone'

async function copyPermissions() {
  error.value = ''
  try {
    await navigator.clipboard.writeText(permissionText)
    message.value = '最小权限清单已复制。'
  } catch {
    error.value = '浏览器不允许自动复制，请按页面清单手动配置。'
  }
}

async function validate() {
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    const result = await api<{ zones: Zone[] }>('/api/v1/cloudflare/validate', {
      method: 'POST', body: JSON.stringify({ token: token.value }),
    })
    zones.value = result.zones
    zoneId.value = result.zones[0]?.id ?? ''
    if (result.zones.length === 0) {
      error.value = 'Token 有效，但未读取到 Zone。请检查 Zone Read 和 Specific zone 资源范围。'
      return
    }
    message.value = `Token 有效，可读取 ${result.zones.length} 个 Zone。请选择一个 Zone 保存。`
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Token 检测失败'
  } finally {
    busy.value = false
  }
}

async function save() {
  if (!selectedZone.value) return
  busy.value = true
  error.value = ''
  try {
    await api('/api/v1/cloudflare/connections', {
      method: 'POST',
      body: JSON.stringify({
        name: name.value, token: token.value, zoneId: selectedZone.value.id, zoneName: selectedZone.value.name,
      }),
    })
    token.value = ''
    zones.value = []
    zoneId.value = ''
    message.value = '连接已加密保存。'
    emit('changed')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '保存失败'
  } finally {
    busy.value = false
  }
}

async function remove(id: string) {
  if (!window.confirm('删除这个 Cloudflare 连接？正在使用它的服务会阻止删除。')) return
  try {
    await api(`/api/v1/cloudflare/connections/${id}`, { method: 'DELETE' })
    emit('changed')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '删除失败'
  }
}
</script>

<template>
  <section class="panel">
    <div class="panel-heading">
      <div><p class="eyebrow">PROVIDER</p><h2>Cloudflare 连接</h2></div>
      <span class="count-chip">{{ connections.length }}</span>
    </div>
    <div v-if="connections.length" class="compact-list">
      <div v-for="connection in connections" :key="connection.id" class="compact-row">
        <div><strong>{{ connection.name }}</strong><small>{{ connection.zoneName }}</small></div>
        <button class="text-button danger" type="button" @click="remove(connection.id)">删除</button>
      </div>
    </div>
    <section id="token-setup" class="token-setup">
      <div class="token-intro">
        <div><span class="step-number">01</span><div><strong>在 Cloudflare 创建 Token</strong><p>打开 API Token 页面，选择“创建自定义 Token”，再按下面清单限制到单个 Zone。</p></div></div>
        <a class="button primary" href="https://dash.cloudflare.com/profile/api-tokens" target="_blank" rel="noreferrer">打开 Token 配置</a>
      </div>
      <div class="permission-card">
        <header><div><p class="eyebrow">LEAST PRIVILEGE</p><h3>StunDeck 最小权限</h3></div><button class="text-button" type="button" @click="copyPermissions">复制清单</button></header>
        <div class="permission-list">
          <p><strong>Zone · Zone · Read</strong><span>读取可选 Zone</span></p>
          <p><strong>Zone · DNS · Edit</strong><span>仅自动管理 DNS 时使用</span></p>
          <p><strong>Zone · Single Redirect · Edit</strong><span>创建与更新跳转规则</span></p>
          <p><strong>Zone Resources · Specific zone</strong><span>不要授权全部 Zone</span></p>
        </div>
        <p class="permission-note">Cloudflare 首个 Token 必须由你在 Dashboard 确认创建。StunDeck 不索取“API Tokens Edit”，避免获得创建任意 Token 的高危权限。</p>
      </div>
      <div class="token-form-heading"><span class="step-number">02</span><div><strong>粘贴并验证</strong><p>Token 只会发送到 Cloudflare 验证，并在本机加密保存。</p></div></div>
      <div class="form-grid">
        <label>连接名称<input v-model.trim="name" maxlength="100" /></label>
        <label class="span-2">Cloudflare API Token<input v-model.trim="token" type="password" autocomplete="off" spellcheck="false" placeholder="不会写入日志或返回给浏览器" /></label>
        <button class="button secondary" type="button" :disabled="busy || !token" @click="validate">验证 Token 与 Zone</button>
        <label v-if="zones.length" class="span-2">Zone
          <select v-model="zoneId"><option v-for="zone in zones" :key="zone.id" :value="zone.id">{{ zone.name }} · {{ zone.status }}</option></select>
        </label>
        <button v-if="zones.length" class="button primary" type="button" :disabled="busy || !zoneId" @click="save">加密保存</button>
      </div>
      <div v-if="connections.length" class="next-step"><span class="step-number">03</span><div><strong>继续创建映射服务</strong><p>Cloudflare 已就绪；创建服务时可选择 Redirect 发布。</p></div><RouterLink class="button secondary" to="/services">进入映射服务</RouterLink></div>
    </section>
    <p v-if="message" class="success-text">{{ message }}</p>
    <p v-if="error" class="error-text">{{ error }}</p>
  </section>
</template>
