import { createApp } from "vue";
import "vuetify/styles";
import { createVuetify } from "vuetify";
import App from "./App.vue";
import "./styles.css";

const vuetify = createVuetify({
  theme: {
    defaultTheme: "nixhostforge",
    themes: {
      nixhostforge: {
        dark: true,
        colors: {
          background: "#080b12",
          surface: "#111827",
          primary: "#8b5cf6",
          secondary: "#06b6d4",
          error: "#ef4444",
          warning: "#f59e0b",
          success: "#22c55e",
        },
      },
    },
  },
});

createApp(App).use(vuetify).mount("#app");
