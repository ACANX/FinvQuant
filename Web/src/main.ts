import 'vuetify/styles'
import '@mdi/font/css/materialdesignicons.css'
import { createApp } from 'vue'
import { createVuetify } from 'vuetify'
import { zhHans } from 'vuetify/locale'

import App from './App.vue'
import { resolveInitialTheme } from './config/uiConfig'
import router from './router'

// Vuetify 4：全局中文（zhHans），日期组件按中国习惯展示简体中文
const vuetify = createVuetify({
  locale: {
    locale: 'zhHans',
    fallback: 'en',
    messages: { zhHans },
  },
  theme: {
    // 初始主题：用户本地选择 > 系统偏好 > 深色；之后由右上角用户菜单开关切换
    defaultTheme: resolveInitialTheme(),
  },
})

createApp(App).use(vuetify).use(router).mount('#app')
