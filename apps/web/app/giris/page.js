import GirisClient from "../../components/GirisClient";

export const metadata = {
  title: "Giriş — içini dök.",
  description:
    "Kullanıcı adın ve authenticator kodunla giriş yap. E-posta ve şifre yok.",
};

export default function GirisPage() {
  return <GirisClient />;
}
