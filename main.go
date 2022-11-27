package main;

import (
  "context"
  "encoding/json"
  "fmt"
  "log"
  "net/http"
  "os"

  "golang.org/x/oauth2"
  "golang.org/x/oauth2/google"
  "google.golang.org/api/drive/v3"
  "google.golang.org/api/option"

  "errors"
);

func getClient(config *oauth2.Config) *http.Client { //U: Retrieve a token, saves the token, then returns the generated client.
  /* NOTE:
  *  The file token.json stores the user's access and refresh tokens, and is
  *  created automatically when the authorization flow completes for the first
  *  time.
  */
  tokFile := "token.json"; //TODO: Different users
  tok, err := tokenFromFile(tokFile);
  if err != nil { //A: First time
    tok = getTokenFromWeb(config);
    saveToken(tokFile, tok);
  }
  return config.Client(context.Background(), tok);
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token { //U: Request a token from the web, then returns the retrieved token.
  authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline);
  fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL);

  var authCode string
  if _, err := fmt.Scan(&authCode); err != nil {
    log.Fatalf("[getTokenFromWeb] Unable to read authorization code %v", err);
  }

  tok, err := config.Exchange(context.TODO(), authCode);
  if err != nil {
    log.Fatalf("[getTokenFromWeb] Unable to retrieve token from web %v", err);
  }

  return tok;
  //NOTE: Token is in the URL; TODO
}

func tokenFromFile(file string) (*oauth2.Token, error) { //U: Retrieves a token from a local file.
  f, err := os.Open(file);
  if err != nil {
    return nil, err;
  }
  defer f.Close();
  tok := &oauth2.Token{};
  err = json.NewDecoder(f).Decode(tok);
  return tok, err;
}

func saveToken(path string, token *oauth2.Token) { //U: Saves a token to a file path.
  fmt.Printf("Saving credential file to: %s\n", path);
  f, err := os.OpenFile(path, os.O_RDWR | os.O_CREATE | os.O_TRUNC, 0600);
  if err != nil {
    log.Fatalf("[saveToken] Unable to cache oauth token: %v", err);
  }
  defer f.Close();
  json.NewEncoder(f).Encode(token);
}

func uploadFile(localPath string, drivePath string, service *drive.Service) error { //U: Uploads a local file to Drive
  file, err := os.Open(localPath);
  if err != nil {
    log.Printf("[uploadFile] Error opening \"%v\": %v", localPath, err);
    return errors.New(fmt.Sprintf("Couldn't open \"%v\"", localPath));
  }

  _, err = service.Files.Create(&drive.File{Name: drivePath}).Media(file).Do();
  if err != nil {
    log.Printf("[uploadFile] Unable to create \"%v\": %v", drivePath, err);
  }

  return nil;
}

func main() {
  ctx := context.Background();
  b, err := os.ReadFile("credentials.json");
  if err != nil {
    log.Fatalf("[main] Unable to read client secret file: %v", err);
  }

  //NOTE: Delete token.json if modifying these scopes
  config, err := google.ConfigFromJSON(b, drive.DriveScope); //A: Full access to Drive
  if err != nil {
    log.Fatalf("[main] Unable to parse client secret file to config: %v", err);
  }
  client := getClient(config);

  srv, err := drive.NewService(ctx, option.WithHTTPClient(client));
  if err != nil {
    log.Fatalf("[main] Unable to retrieve Drive client: %v", err);
  }

  err = uploadFile("test", "test", srv);
  if err != nil {
    log.Fatalf("[main] Unable to upload file: %v", err);
  }
  
  r, err := srv.Files.List().PageSize(10).
              Fields("nextPageToken, files(id, name)").Do();
  if err != nil {
    log.Fatalf("[main] Unable to retrieve files: %v", err);
  }
  fmt.Println("Files:");
  if len(r.Files) == 0 {
    fmt.Println("No files found.");
  } else {
    for _, i := range r.Files {
      fmt.Printf("%s (%s)\n", i.Name, i.Id);
    }
  }
}

//TODO: Log vs user print
