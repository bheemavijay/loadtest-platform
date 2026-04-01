/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {
      colors: {
        slateNight: '#0e1726',
        ink: '#122033',
        mist: '#e8f0ff',
        mint: '#4ade80',
        coral: '#fb7185',
        sun: '#fbbf24',
        aqua: '#38bdf8'
      },
      fontFamily: {
        sans: ['Manrope', 'ui-sans-serif', 'system-ui', 'sans-serif']
      },
      boxShadow: {
        panel: '0 20px 60px rgba(15, 23, 42, 0.18)'
      },
      backgroundImage: {
        grid: 'linear-gradient(rgba(148,163,184,0.08) 1px, transparent 1px), linear-gradient(90deg, rgba(148,163,184,0.08) 1px, transparent 1px)'
      }
    }
  },
  plugins: []
};
