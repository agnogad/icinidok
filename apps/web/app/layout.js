import "./globals.css";

export const metadata = {
  title: "içini dök.",
  description:
    "Derdini anonim olarak paylaş; okuyan biriyle 24 saatlik sakin bir sohbet aç.",
};

const themeScript = `try{var t=localStorage.getItem("icini-theme");if(t==="dark")document.documentElement.dataset.theme="dark";}catch(e){}`;

export default function RootLayout({ children }) {
  return (
    <html lang="tr" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>{children}</body>
    </html>
  );
}
