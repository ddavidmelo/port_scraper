FROM golang:1.19-alpine

ENV     PROJECT_PATH=/port_scraper

RUN     mkdir -p $PROJECT_PATH

COPY    .  $PROJECT_PATH
WORKDIR $PROJECT_PATH

RUN go mod download
RUN go build -o /port_scraper .
RUN chmod +x /port_scraper 

CMD [ "./port_scraper" ]